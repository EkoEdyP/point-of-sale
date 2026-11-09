package security

import (
	"errors"
	"time"

	"github.com/EkoEdyP/point-of-sale/config"
	"github.com/golang-jwt/jwt/v5"
)

// AccessClaims hanya membawa id pengguna (Subject) dan masa berlaku. Peran dan status aktif sengaja tidak ikut:
// middleware membacanya dari database di setiap request, supaya perubahan peran atau penonaktifan langsung berlaku.
type AccessClaims struct {
	jwt.RegisteredClaims
}

func GenerateAccessToken(cfg config.JWTConfig, userId string) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Subject:   userId,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(cfg.ExpiresInMinutes) * time.Minute)),
		},
	}
	return jwt.NewWithClaims(cfg.SigningMethod, claims).SignedString(cfg.SignatureKey)
}

// ParseAccessToken hanya menerima token seperti buatan GenerateAccessToken: algoritma dan issuer sama, exp dan iat
// wajib ada, dan umurnya tidak lebih dari ExpiresInMinutes. Token tanpa batas waktu ditolak walau tanda tangannya
// cocok.
func ParseAccessToken(cfg config.JWTConfig, tokenString string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&AccessClaims{},
		func(t *jwt.Token) (any, error) { return cfg.SignatureKey, nil },
		jwt.WithValidMethods([]string{cfg.SigningMethod.Alg()}),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	// Satu menit kelonggaran supaya pembulatan detik tidak pernah menolak token yang sah.
	maxLifetime := time.Duration(cfg.ExpiresInMinutes)*time.Minute + time.Minute
	if claims.IssuedAt == nil || claims.ExpiresAt.Sub(claims.IssuedAt.Time) > maxLifetime {
		return nil, errors.New("token lifetime exceeds the configured limit")
	}
	return claims, nil
}
