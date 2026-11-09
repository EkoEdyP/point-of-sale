// Membuat akun admin pertama lewat CLI.
//
//	make seed-admin USERNAME=admin NAME="Admin Toko"
//	(atau) go run ./cmd/seed-admin --username admin --name "Admin Toko"
//
// Password diminta lewat prompt (tidak tampil di layar). Untuk skrip otomatis,
// isi env POS_ADMIN_PASSWORD. Jangan menulis password di kode atau di repo.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/EkoEdyP/point-of-sale/config"
	"github.com/EkoEdyP/point-of-sale/manager"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/utility/common"
	"github.com/EkoEdyP/point-of-sale/utility/security"
	"github.com/lib/pq"
	"golang.org/x/term"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72 // batas bcrypt
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	username := flag.String("username", "", "username admin (wajib)")
	name := flag.String("name", "", "nama lengkap (default: sama dengan username)")
	flag.Parse()

	*username = strings.TrimSpace(*username)
	if *username == "" {
		return errors.New("--username wajib diisi")
	}
	if strings.TrimSpace(*name) == "" {
		*name = *username
	}

	password, err := readPassword()
	if err != nil {
		return err
	}

	cfg, err := config.NewConfig()
	if err != nil {
		return err
	}
	infraManager, err := manager.NewInfraManager(cfg)
	if err != nil {
		return err
	}
	userRepository := manager.NewRepositoryManager(infraManager).UserRepository()

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}

	user := model.User{
		Id:           common.GenerateID(),
		Name:         *name,
		Username:     *username,
		PasswordHash: hash,
		Role:         model.RoleAdmin,
	}
	if err := userRepository.Save(user); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return fmt.Errorf("username %q sudah dipakai", *username)
		}
		return fmt.Errorf("menyimpan user: %w", err)
	}

	fmt.Printf("admin %q dibuat (id: %s)\n", *username, user.Id)
	return nil
}

func readPassword() (string, error) {
	if p := os.Getenv("POS_ADMIN_PASSWORD"); p != "" {
		return validate(p)
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("stdin bukan terminal; isi env POS_ADMIN_PASSWORD")
	}

	fmt.Fprint(os.Stderr, "Password: ")
	p1, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Ulangi password: ")
	p2, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(p1) != string(p2) {
		return "", errors.New("password tidak sama")
	}
	return validate(string(p1))
}

func validate(p string) (string, error) {
	if len(p) < minPasswordLen {
		return "", fmt.Errorf("password minimal %d karakter", minPasswordLen)
	}
	if len(p) > maxPasswordLen {
		return "", fmt.Errorf("password maksimal %d karakter", maxPasswordLen)
	}
	return p, nil
}
