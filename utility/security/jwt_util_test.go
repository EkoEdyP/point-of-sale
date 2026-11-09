package security

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/EkoEdyP/point-of-sale/config"
	"github.com/golang-jwt/jwt/v5"
)

func testJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Issuer:           "pos-uji",
		SignatureKey:     []byte("kunci-uji-hanya-untuk-tes-0123456789abcdef0123456789abcdef"),
		SigningMethod:    jwt.SigningMethodHS512,
		ExpiresInMinutes: 15,
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	cfg := testJWTConfig()
	token, err := GenerateAccessToken(cfg, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAccessToken(cfg, token)
	if err != nil {
		t.Fatalf("token buatan server ditolak: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("Subject = %q", claims.Subject)
	}
	// Peran tidak ikut di token: middleware membacanya dari database di setiap request.
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"role"`) {
		t.Fatalf("token masih membawa role: %s", payload)
	}
}

func TestParseAccessTokenRejects(t *testing.T) {
	cfg := testJWTConfig()
	now := time.Now()
	lifetime := time.Duration(cfg.ExpiresInMinutes) * time.Minute
	date := jwt.NewNumericDate
	sign := func(key []byte, iat, exp *jwt.NumericDate) string {
		c := jwt.RegisteredClaims{Issuer: cfg.Issuer, Subject: "user-1", IssuedAt: iat, ExpiresAt: exp}
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS512, c).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	rejected := map[string]string{
		"tanpa exp":         sign(cfg.SignatureKey, date(now), nil),
		"tanpa iat":         sign(cfg.SignatureKey, nil, date(now.Add(lifetime))),
		"berumur 10 tahun":  sign(cfg.SignatureKey, date(now), date(now.AddDate(10, 0, 0))),
		"iat di masa depan": sign(cfg.SignatureKey, date(now.Add(time.Hour)), date(now.Add(time.Hour+lifetime))),
		"kunci lain":        sign([]byte("kunci-lain-0123456789abcdef0123456789abcdef"), date(now), date(now.Add(lifetime))),
		"bukan token":       "abc.def.ghi",
	}
	for name, token := range rejected {
		if _, err := ParseAccessToken(cfg, token); err == nil {
			t.Errorf("token %s diterima", name)
		}
	}
	if _, err := ParseAccessToken(cfg, sign(cfg.SignatureKey, date(now), date(now.Add(lifetime)))); err != nil {
		t.Errorf("token berumur tepat %v ditolak: %v", lifetime, err)
	}
}
