package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type (
	DbConfig struct {
		Driver   string
		Host     string
		Port     string
		User     string
		Password string
		Name     string
		SSLMode  string
		TimeZone string
		// Additional configurations⤵️
		MaxConns         int           // batas koneksi pool sekaligus (APP_DB_MAX_CONNS, bawaan 20)
		StatementTimeout time.Duration // query yang lebih lama dibatalkan (APP_DB_STATEMENT_TIMEOUT, bawaan 30s)
		LockTimeout      time.Duration // batas menunggu baris yang dikunci transaksi lain (APP_DB_LOCK_TIMEOUT, 10s)
	}

	ApiConfig struct {
		ApiHost string
		ApiPort string
		// Additional configurations⤵️
		WebDir string // folder hasil build tampilan web yang ikut disajikan (APP_WEB_DIR, bawaan web/dist)
	}

	FileLogConfig struct {
		FilePath string
		// Additional configurations⤵️
		LogLevel logrus.Level // APP_LOG_LEVEL: debug, info (bawaan), warn, error
	}

	JWTConfig struct {
		Issuer               string
		SignatureKey         []byte
		SigningMethod        *jwt.SigningMethodHMAC
		ExpiresInMinutes     int // masa berlaku access token
		RefreshExpiresInDays int // masa berlaku refresh token
		// Additional configurations⤵️
	}
)

type Config struct {
	DbConfig
	ApiConfig
	FileLogConfig
	JWTConfig
}

// envFile: file pengaturan yang di-source di Mac/Linux (baris export NAMA=nilai). Server juga membacanya sendiri dari
// folder kerja, supaya bisa dijalankan tanpa shell, misalnya lewat start.bat di Windows. Variabel lingkungan yang
// sudah diset tetap menang, jadi nilai dari Makefile atau baris perintah (APP_DB_NAME=pos_check ...) tidak tertimpa.
const envFile = "env.sh"

// defaultWebDir: hasil npm run build di web/, relatif terhadap folder kerja.
const defaultWebDir = "web/dist"

func (c *Config) LoadConfig() error {
	v := viper.New()
	v.AutomaticEnv()
	if err := readEnvFile(v, envFile); err != nil {
		return err
	}

	c.DbConfig = DbConfig{
		Driver:   v.GetString("APP_DB_DRIVER"),
		Host:     v.GetString("APP_DB_HOST"),
		Port:     v.GetString("APP_DB_PORT"),
		User:     v.GetString("APP_DB_USER"),
		Password: v.GetString("APP_DB_PASSWORD"),
		Name:     v.GetString("APP_DB_NAME"),
		SSLMode:  v.GetString("APP_SSL_MODE"),
		TimeZone: v.GetString("APP_TIME_ZONE"),
	}

	c.ApiConfig = ApiConfig{
		ApiHost: v.GetString("APP_API_HOST"),
		ApiPort: v.GetString("APP_API_PORT"),
		WebDir:  strings.TrimSpace(v.GetString("APP_WEB_DIR")),
	}

	c.FileLogConfig = FileLogConfig{
		FilePath: v.GetString("APP_FILE_PATH"),
	}

	c.JWTConfig = JWTConfig{
		Issuer:               v.GetString("APP_ISSUER"),
		SignatureKey:         []byte(v.GetString("APP_SIGNATURE_KEY")),
		SigningMethod:        jwt.SigningMethodHS512,
		ExpiresInMinutes:     v.GetInt("APP_EXPIRES_IN_MINUTES"),
		RefreshExpiresInDays: v.GetInt("APP_REFRESH_EXPIRES_IN_DAYS"),
	}

	var missing []string
	check := func(name string, empty bool) {
		if empty {
			missing = append(missing, name)
		}
	}
	check("APP_DB_DRIVER", c.DbConfig.Driver == "")
	check("APP_DB_HOST", c.DbConfig.Host == "")
	check("APP_DB_PORT", c.DbConfig.Port == "")
	check("APP_DB_USER", c.DbConfig.User == "")
	check("APP_DB_PASSWORD", c.DbConfig.Password == "")
	check("APP_DB_NAME", c.DbConfig.Name == "")
	check("APP_SSL_MODE", c.DbConfig.SSLMode == "")
	check("APP_TIME_ZONE", c.DbConfig.TimeZone == "")
	check("APP_API_HOST", c.ApiConfig.ApiHost == "")
	check("APP_API_PORT", c.ApiConfig.ApiPort == "")
	check("APP_FILE_PATH", c.FileLogConfig.FilePath == "")
	check("APP_ISSUER", c.JWTConfig.Issuer == "")
	check("APP_SIGNATURE_KEY", len(c.JWTConfig.SignatureKey) == 0)
	check("APP_EXPIRES_IN_MINUTES", c.JWTConfig.ExpiresInMinutes == 0)
	check("APP_REFRESH_EXPIRES_IN_DAYS", c.JWTConfig.RefreshExpiresInDays == 0)
	if len(missing) > 0 {
		return fmt.Errorf("missing required env var: %s", strings.Join(missing, ", "))
	}

	if len(c.JWTConfig.SignatureKey) < 32 {
		return fmt.Errorf("APP_SIGNATURE_KEY minimal 32 karakter")
	}
	// Kunci contoh lama dari env.example.sh berbunyi "ganti...". Kunci itu tersebar bersama kode, jadi siapa pun
	// yang membacanya bisa membuat token admin.
	if strings.Contains(strings.ToLower(string(c.JWTConfig.SignatureKey)), "ganti") {
		return fmt.Errorf("APP_SIGNATURE_KEY masih nilai contoh, buat kunci baru dengan: openssl rand -hex 32")
	}

	// Opsional: kosong berarti nilai bawaan.
	if c.ApiConfig.WebDir == "" {
		c.ApiConfig.WebDir = defaultWebDir
	}
	var err error
	if c.DbConfig.MaxConns, err = optionalInt(v, "APP_DB_MAX_CONNS", 20, 1, 100); err != nil {
		return err
	}
	if c.DbConfig.StatementTimeout, err = optionalDuration(v, "APP_DB_STATEMENT_TIMEOUT", 30*time.Second); err != nil {
		return err
	}
	if c.DbConfig.LockTimeout, err = optionalDuration(v, "APP_DB_LOCK_TIMEOUT", 10*time.Second); err != nil {
		return err
	}
	level := strings.TrimSpace(v.GetString("APP_LOG_LEVEL"))
	if level == "" {
		level = "info"
	}
	if c.FileLogConfig.LogLevel, err = logrus.ParseLevel(level); err != nil {
		return fmt.Errorf("APP_LOG_LEVEL harus salah satu dari: debug, info, warn, error")
	}
	return nil
}

// readEnvFile memakai isi file env name sebagai nilai di bawah variabel lingkungan. File yang tidak ada dilewati.
func readEnvFile(v *viper.Viper, name string) error {
	if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	v.SetConfigFile(name)
	v.SetConfigType("env")
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("baca %s: %w", name, err)
	}
	return nil
}

func optionalInt(v *viper.Viper, name string, fallback, lo, hi int) (int, error) {
	s := strings.TrimSpace(v.GetString(name))
	if s == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("%s harus angka %d sampai %d", name, lo, hi)
	}
	return n, nil
}

func optionalDuration(v *viper.Viper, name string, fallback time.Duration) (time.Duration, error) {
	s := strings.TrimSpace(v.GetString(name))
	if s == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < time.Millisecond {
		return 0, fmt.Errorf("%s harus berupa durasi, misalnya 30s atau 2m", name)
	}
	return d, nil
}

// NewConfig Constructor
func NewConfig() (*Config, error) {
	cfg := &Config{}
	err := cfg.LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
