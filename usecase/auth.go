package usecase

import (
	"database/sql"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/EkoEdyP/point-of-sale/config"
	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/EkoEdyP/point-of-sale/utility/common"
	"github.com/EkoEdyP/point-of-sale/utility/security"
	"github.com/go-playground/validator/v10"
	"github.com/lib/pq"
)

var (
	errInvalidCredentials = &exception.GetError{Code: http.StatusUnauthorized, Message: "username atau password salah"}
	errInvalidSession     = &exception.GetError{Code: http.StatusUnauthorized, Message: "sesi tidak valid, silakan login ulang"}
	// Refresh token ini baru saja diputar request lain, misalnya tab lain di browser yang sama. Cookie barunya sudah
	// dikirim ke browser itu, jadi client cukup mengulang; ini bukan tanda token dicuri.
	errSessionRotated = &exception.GetError{Code: http.StatusConflict, Message: "sesi baru diperbarui, coba lagi"}
)

// rotationGrace: refresh token yang dipakai lagi dalam selang ini setelah diputar hanya ditolak (errSessionRotated).
// Selang ini cukup untuk request yang berangkat sebelum cookie barunya tiba. Di luar selang ini pemakaian ulang
// dianggap pencurian dan semua sesi user dicabut.
const rotationGrace = 10 * time.Second

// AuthResult dipakai controller: Response dikirim sebagai JSON,
// RefreshToken dikirim sebagai cookie HttpOnly.
type AuthResult struct {
	Response         res.AuthResponse
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type (
	AuthUseCase interface {
		Login(request req.LoginRequest) (AuthResult, error)
		Register(actorId string, request req.RegisterRequest) (res.UserResponse, error)
		Refresh(rawRefreshToken string) (AuthResult, error)
		Logout(rawRefreshToken string) error
		// DeleteExpiredSessions menghapus sesi login (refresh token) yang sudah kedaluwarsa, dijalankan server
		// berkala. Mengembalikan jumlah yang dihapus.
		DeleteExpiredSessions() (int64, error)
	}
	authUseCase struct {
		users    postgres.UserRepository
		tokens   postgres.RefreshTokenRepository
		jwtCfg   config.JWTConfig
		validate *validator.Validate
	}
)

func (u *authUseCase) Login(request req.LoginRequest) (AuthResult, error) {
	request.Username = strings.TrimSpace(request.Username)
	if err := u.validate.Struct(request); err != nil {
		return AuthResult{}, validationError(err)
	}

	key := loginKey(request.Username)
	wait, reserved := logins.begin(key)
	if wait > 0 {
		return AuthResult{}, loginThrottled(wait)
	}
	user, known, err := u.checkCredentials(request)
	outcome := loginNeutral
	switch {
	case err == nil:
		outcome = loginSucceeded
	case errors.Is(err, errInvalidCredentials):
		outcome = loginFailed
	}
	logins.done(key, reserved, outcome, known)
	if err != nil {
		return AuthResult{}, err
	}

	accessToken, err := security.GenerateAccessToken(u.jwtCfg, user.Id)
	if err != nil {
		return AuthResult{}, internalError(err)
	}
	raw, hash, err := security.GenerateRefreshToken()
	if err != nil {
		return AuthResult{}, internalError(err)
	}
	expiresAt := time.Now().Add(time.Duration(u.jwtCfg.RefreshExpiresInDays) * 24 * time.Hour)
	err = u.tokens.Save(model.RefreshToken{
		Id:        common.GenerateID(),
		UserId:    user.Id,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return AuthResult{}, internalError(err)
	}

	return u.result(user, accessToken, raw, expiresAt), nil
}

// dummyPasswordHash dibandingkan saat username tidak ada, supaya jawabannya sama lambatnya dengan password salah.
var dummyPasswordHash = sync.OnceValue(func() string {
	hash, err := security.HashPassword("akun-tidak-ada")
	if err != nil {
		return ""
	}
	return hash
})

// checkCredentials memberi pesan yang sama, dan memakai waktu yang kira-kira sama (satu perbandingan bcrypt), untuk
// akun yang tidak ada, nonaktif, atau password salah, supaya username yang ada tidak bisa ditebak. known: username
// itu ada di database.
func (u *authUseCase) checkCredentials(request req.LoginRequest) (user model.User, known bool, err error) {
	user, err = u.users.FindByUsername(request.Username)
	if errors.Is(err, sql.ErrNoRows) {
		security.CheckPassword(dummyPasswordHash(), request.Password)
		return model.User{}, false, errInvalidCredentials
	}
	if err != nil {
		return model.User{}, false, internalError(err)
	}
	if !security.CheckPassword(user.PasswordHash, request.Password) || !user.Active {
		return model.User{}, true, errInvalidCredentials
	}
	return user, true, nil
}

func (u *authUseCase) Register(actorId string, request req.RegisterRequest) (res.UserResponse, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Username = strings.TrimSpace(request.Username)
	if err := u.validate.Struct(request); err != nil {
		return res.UserResponse{}, validationError(err)
	}

	hash, err := security.HashPassword(request.Password)
	if err != nil {
		return res.UserResponse{}, internalError(err)
	}

	user := model.User{
		Id:           common.GenerateID(),
		Name:         request.Name,
		Username:     request.Username,
		PasswordHash: hash,
		Role:         request.Role,
		Active:       true,
		CreatedBy:    &actorId,
	}
	if err := u.users.Save(user); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return res.UserResponse{}, &exception.GetError{Code: http.StatusConflict, Message: "username sudah dipakai"}
		}
		return res.UserResponse{}, internalError(err)
	}
	return toUserResponse(user), nil
}

// Refresh menukar refresh token dengan access token baru, sekaligus memutar (rotate) refresh token.
func (u *authUseCase) Refresh(rawRefreshToken string) (AuthResult, error) {
	if rawRefreshToken == "" {
		return AuthResult{}, errInvalidSession
	}

	stored, err := u.tokens.FindByHash(security.HashToken(rawRefreshToken))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthResult{}, errInvalidSession
		}
		return AuthResult{}, internalError(err)
	}

	// Token yang sudah dicabut dipakai lagi. Baru saja diputar (tab lain): tolak saja. Selain itu anggap dicuri dan
	// cabut semua sesi user ini.
	if stored.RevokedAt != nil {
		if stored.ReplacedBy != nil && time.Since(*stored.RevokedAt) < rotationGrace {
			return AuthResult{}, errSessionRotated
		}
		_ = u.tokens.RevokeAllByUser(stored.UserId)
		return AuthResult{}, errInvalidSession
	}
	if time.Now().After(stored.ExpiresAt) {
		return AuthResult{}, errInvalidSession
	}

	user, err := u.users.FindById(stored.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthResult{}, errInvalidSession
		}
		return AuthResult{}, internalError(err)
	}
	if !user.Active {
		_ = u.tokens.RevokeAllByUser(user.Id)
		return AuthResult{}, errInvalidSession
	}

	accessToken, err := security.GenerateAccessToken(u.jwtCfg, user.Id)
	if err != nil {
		return AuthResult{}, internalError(err)
	}
	raw, hash, err := security.GenerateRefreshToken()
	if err != nil {
		return AuthResult{}, internalError(err)
	}
	expiresAt := time.Now().Add(time.Duration(u.jwtCfg.RefreshExpiresInDays) * 24 * time.Hour)
	err = u.tokens.Rotate(stored.Id, model.RefreshToken{
		Id:        common.GenerateID(),
		UserId:    user.Id,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		// Request lain memutar token yang sama tepat bersamaan (dua tab refresh sekaligus).
		if errors.Is(err, postgres.ErrTokenAlreadyRevoked) {
			return AuthResult{}, errSessionRotated
		}
		return AuthResult{}, internalError(err)
	}

	return u.result(user, accessToken, raw, expiresAt), nil
}

func (u *authUseCase) Logout(rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	if err := u.tokens.RevokeByHash(security.HashToken(rawRefreshToken)); err != nil {
		return internalError(err)
	}
	return nil
}

func (u *authUseCase) DeleteExpiredSessions() (int64, error) {
	n, err := u.tokens.DeleteExpired()
	if err != nil {
		return 0, internalError(err)
	}
	return n, nil
}

func (u *authUseCase) result(user model.User, accessToken, rawRefresh string, refreshExpiresAt time.Time) AuthResult {
	return AuthResult{
		Response: res.AuthResponse{
			AccessToken: accessToken,
			TokenType:   "Bearer",
			ExpiresIn:   u.jwtCfg.ExpiresInMinutes * 60,
			User:        toUserResponse(user),
		},
		RefreshToken:     rawRefresh,
		RefreshExpiresAt: refreshExpiresAt,
	}
}

func toUserResponse(user model.User) res.UserResponse {
	return res.UserResponse{
		Id:       user.Id,
		Name:     user.Name,
		Username: user.Username,
		Role:     user.Role,
		Active:   user.Active,
	}
}

func internalError(err error) error {
	return exception.Internal(err)
}

func validationError(err error) error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		parts := make([]string, 0, len(ve))
		for _, fe := range ve {
			parts = append(parts, fe.Field()+" tidak valid ("+fe.Tag()+")")
		}
		return &exception.GetError{Code: http.StatusBadRequest, Message: strings.Join(parts, ", ")}
	}
	return internalError(err)
}

func newValidator() *validator.Validate {
	v := validator.New()
	// Pakai nama field dari tag json di pesan error.
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
	return v
}

// NewAuthUseCase Constructor
func NewAuthUseCase(users postgres.UserRepository, tokens postgres.RefreshTokenRepository, jwtCfg config.JWTConfig) AuthUseCase {
	return &authUseCase{
		users:    users,
		tokens:   tokens,
		jwtCfg:   jwtCfg,
		validate: newValidator(),
	}
}
