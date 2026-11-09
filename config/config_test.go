package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// validEnv: semua nilai wajib, dalam urutan env.example.sh.
var validEnv = [][2]string{
	{"APP_DB_DRIVER", "postgres"},
	{"APP_DB_HOST", "localhost"},
	{"APP_DB_PORT", "5432"},
	{"APP_DB_USER", "kasir"},
	{"APP_DB_PASSWORD", "rahasia123"},
	{"APP_DB_NAME", "pos"},
	{"APP_SSL_MODE", "disable"},
	{"APP_TIME_ZONE", "Asia/Jakarta"},
	{"APP_API_HOST", "0.0.0.0"},
	{"APP_API_PORT", "8080"},
	{"APP_FILE_PATH", "logger.log"},
	{"APP_ISSUER", "POS"},
	{"APP_SIGNATURE_KEY", strings.Repeat("a1", 32)},
	{"APP_EXPIRES_IN_MINUTES", "15"},
	{"APP_REFRESH_EXPIRES_IN_DAYS", "7"},
}

// clearAppEnv menghapus semua variabel APP_* selama tes. make test meng-export isi env.sh milik pengembang, dan nilai
// itu akan menang atas file yang sedang dites.
func clearAppEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(name, "APP_") {
			continue
		}
		t.Setenv(name, "") // dikembalikan setelah tes
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

// writeEnvFile menulis env.sh di folder kerja seperti salinan env.example.sh, diawali prefix (misalnya BOM), dengan
// akhir baris eol.
func writeEnvFile(t *testing.T, pairs [][2]string, prefix, eol string) {
	t.Helper()
	lines := []string{"#!/bin/bash", "# Database Configuration"}
	for _, p := range pairs {
		lines = append(lines, "export "+p[0]+"="+p[1])
	}
	lines = append(lines, "# export APP_DB_MAX_CONNS=50")
	if err := os.WriteFile(envFile, []byte(prefix+strings.Join(lines, eol)+eol), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigReadsEnvFile(t *testing.T) {
	for _, tc := range []struct{ name, prefix, eol string }{
		{"LF", "", "\n"},
		{"CRLF", "", "\r\n"},
		{"BOM dan CRLF dari Notepad lama", "\xEF\xBB\xBF", "\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearAppEnv(t)
			t.Chdir(t.TempDir())
			writeEnvFile(t, validEnv, tc.prefix, tc.eol)
			t.Setenv("APP_DB_NAME", "pos_check")
			checkLoadedEnvFile(t)
		})
	}
}

func checkLoadedEnvFile(t *testing.T) {
	t.Helper()
	cfg, err := NewConfig()
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name, got, want string
	}{
		{"user dari file", cfg.DbConfig.User, "kasir"},
		{"password dari file", cfg.DbConfig.Password, "rahasia123"},
		{"variabel lingkungan menang", cfg.DbConfig.Name, "pos_check"},
		{"host", cfg.ApiConfig.ApiHost, "0.0.0.0"},
		{"port tanpa CR", cfg.ApiConfig.ApiPort, "8080"},
		{"folder web bawaan", cfg.ApiConfig.WebDir, defaultWebDir},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: %q, mau %q", c.name, c.got, c.want)
		}
	}
	if len(cfg.JWTConfig.SignatureKey) != 64 {
		t.Errorf("panjang kunci %d, mau 64", len(cfg.JWTConfig.SignatureKey))
	}
	if cfg.DbConfig.MaxConns != 20 {
		t.Errorf("baris yang dikomentari ikut terbaca: MaxConns = %d", cfg.DbConfig.MaxConns)
	}
}

func TestLoadConfigWithoutEnvFile(t *testing.T) {
	clearAppEnv(t)
	t.Chdir(t.TempDir())
	for _, p := range validEnv {
		t.Setenv(p[0], p[1])
	}
	t.Setenv("APP_WEB_DIR", "C:/pos/web/dist")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DbConfig.User != "kasir" || cfg.ApiConfig.WebDir != "C:/pos/web/dist" {
		t.Errorf("user %q, folder web %q", cfg.DbConfig.User, cfg.ApiConfig.WebDir)
	}
}

func TestLoadConfigMissingValues(t *testing.T) {
	clearAppEnv(t)
	t.Chdir(t.TempDir())
	if _, err := NewConfig(); err == nil || !strings.Contains(err.Error(), "missing required env var") {
		t.Fatalf("error %v, mau daftar variabel yang kurang", err)
	}
}

// Contoh di repo disalin menjadi env.sh, jadi harus tetap bisa dibaca server.
func TestEnvExampleIsReadable(t *testing.T) {
	clearAppEnv(t)
	v := viper.New()
	if err := readEnvFile(v, filepath.Join("..", "env.example.sh")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"APP_DB_HOST", "APP_API_PORT", "APP_FILE_PATH", "APP_ISSUER"} {
		if v.GetString(name) == "" {
			t.Errorf("%s kosong", name)
		}
	}
	if v.GetString("APP_SIGNATURE_KEY") != "" {
		t.Error("contoh tidak boleh berisi kunci")
	}
	if v.IsSet("APP_DB_MAX_CONNS") || v.IsSet("APP_WEB_DIR") {
		t.Error("baris yang dikomentari ikut terbaca")
	}
}
