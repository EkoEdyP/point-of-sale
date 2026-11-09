package usecase

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/EkoEdyP/point-of-sale/utility/security"
)

var errUserNotFound = &exception.GetError{Code: http.StatusNotFound, Message: "pengguna tidak ditemukan"}

type (
	UserUseCase interface {
		List() (res.UserListResponse, error)
		Update(actorId, id string, request req.UserUpdateRequest) (res.UserResponse, error)
	}
	userUseCase struct {
		users  postgres.UserRepository
		tokens postgres.RefreshTokenRepository
	}
)

func (u *userUseCase) List() (res.UserListResponse, error) {
	users, err := u.users.List()
	if err != nil {
		return res.UserListResponse{}, internalError(err)
	}
	data := make([]res.UserResponse, 0, len(users))
	for _, user := range users {
		data = append(data, toUserResponse(user))
	}
	return res.UserListResponse{Data: data}, nil
}

// Update: admin mengubah nama, status aktif, atau password. Role tidak bisa diubah.
// Menonaktifkan akun atau mengganti password mencabut semua sesi (refresh token) user itu.
func (u *userUseCase) Update(actorId, id string, request req.UserUpdateRequest) (res.UserResponse, error) {
	if !validUUID(id) {
		return res.UserResponse{}, errUserNotFound
	}

	// Validasi manual: validator memperlakukan pointer ke "" sebagai kosong dan melewatinya,
	// padahal password kosong tidak boleh lolos.
	if request.Name == nil && request.Active == nil && request.Password == nil {
		return res.UserResponse{}, badRequest("tidak ada field yang diubah")
	}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" || utf8.RuneCountInString(name) > 100 {
			return res.UserResponse{}, badRequest("name tidak valid (1-100 karakter)")
		}
		request.Name = &name
	}
	if request.Password != nil {
		if len(*request.Password) < 8 {
			return res.UserResponse{}, badRequest("password minimal 8 karakter")
		}
		if len(*request.Password) > 72 {
			return res.UserResponse{}, badRequest("password maksimal 72 karakter")
		}
	}

	if actorId == id && request.Active != nil && !*request.Active {
		return res.UserResponse{}, unprocessable("tidak dapat menonaktifkan akun sendiri")
	}

	// Access token admin yang sudah dinonaktifkan masih berlaku sampai habis, jadi cek ulang ke database.
	actor, err := u.users.FindById(actorId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res.UserResponse{}, errForbidden
		}
		return res.UserResponse{}, internalError(err)
	}
	if !actor.Active || actor.Role != model.RoleAdmin {
		return res.UserResponse{}, errForbidden
	}

	patch := model.UserPatch{Name: request.Name, Active: request.Active}
	if request.Password != nil {
		hash, err := security.HashPassword(*request.Password)
		if err != nil {
			return res.UserResponse{}, internalError(err)
		}
		patch.PasswordHash = &hash
	}

	updated, err := u.users.Update(id, patch, postgres.Audit{ActorId: actorId})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res.UserResponse{}, errUserNotFound
		}
		return res.UserResponse{}, internalError(err)
	}

	deactivated := request.Active != nil && !*request.Active
	if deactivated || request.Password != nil {
		if err := u.tokens.RevokeAllByUser(id); err != nil {
			return res.UserResponse{}, internalError(err)
		}
	}
	return toUserResponse(updated), nil
}

var errForbidden = &exception.GetError{Code: http.StatusForbidden, Message: "akses ditolak"}

// NewUserUseCase Constructor
func NewUserUseCase(users postgres.UserRepository, tokens postgres.RefreshTokenRepository) UserUseCase {
	return &userUseCase{
		users:  users,
		tokens: tokens,
	}
}
