package manager

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/EkoEdyP/point-of-sale/config"
	_ "github.com/lib/pq"
)

type (
	InfraManager interface {
		Conn() *sql.DB
	}

	infraManager struct {
		DB  *sql.DB
		Cfg *config.Config
	}
)

// idleInTransactionTimeout: sesi yang membiarkan transaksi terbuka tanpa query selama ini ditutup Postgres, supaya
// kunci barisnya tidak tertahan karena bug.
const idleInTransactionTimeout = time.Minute

func (i *infraManager) initDb() error {
	cfg := i.Cfg.DbConfig
	// Batas waktu di sisi Postgres. Repository belum memakai context, jadi inilah yang menjamin satu query macet atau
	// kunci yang tertahan tidak menahan request lain selamanya: statement_timeout membatalkan query yang terlalu
	// lama, lock_timeout membatalkan penantian baris yang dikunci transaksi lain. Client menerima 503 dan bisa
	// mencoba lagi. lib/pq meneruskan parameter yang tidak dikenalnya ke Postgres sebagai setelan sesi.
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s "+
		"application_name=pos-api connect_timeout=5 statement_timeout=%d lock_timeout=%d "+
		"idle_in_transaction_session_timeout=%d",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
		cfg.TimeZone,
		cfg.StatementTimeout.Milliseconds(),
		cfg.LockTimeout.Milliseconds(),
		idleInTransactionTimeout.Milliseconds(),
	)
	db, err := sql.Open(cfg.Driver, dsn)
	if err != nil {
		return err
	}
	// Pool dibatasi supaya lonjakan request mengantre di aplikasi, bukan menghabiskan koneksi Postgres (bawaannya
	// max_connections = 100). Koneksi menganggur ditutup setelah 5 menit, dan tiap koneksi diganti setelah 30 menit.
	db.SetMaxOpenConns(cfg.MaxConns)
	db.SetMaxIdleConns(cfg.MaxConns)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	// sql.Open tidak benar-benar menyambung, jadi ping supaya salah konfigurasi ketahuan saat start
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	i.DB = db
	fmt.Printf("successfully connected to database: 🚀%s🚀\n", i.Cfg.DbConfig.Name)

	return nil
}

func (i *infraManager) Conn() *sql.DB {
	return i.DB
}

var (
	instance InfraManager
	once     sync.Once
	initErr  error
)

// NewInfraManager constructor
func NewInfraManager(cfg *config.Config) (InfraManager, error) {
	once.Do(func() {
		conn := &infraManager{
			Cfg: cfg,
		}
		if err := conn.initDb(); err != nil {
			initErr = err
			return
		}
		instance = conn
	})
	return instance, initErr
}
