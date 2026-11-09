package delivery

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/EkoEdyP/point-of-sale/config"
	"github.com/EkoEdyP/point-of-sale/delivery/controller"
	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/manager"
	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
)

// Batas waktu koneksi HTTP. readTimeout membatasi lama client mengirim satu request (koneksi yang sengaja dibuat
// lambat tidak bisa menahan server), writeTimeout lama mengirim response, idleTimeout lama koneksi keep-alive
// menganggur. shutdownTimeout: saat server dihentikan (Ctrl+C), request yang sedang berjalan ditunggu selesai paling
// lama selama ini, supaya penjualan yang sedang disimpan tidak terputus di tengah.
const (
	readTimeout     = 15 * time.Second
	writeTimeout    = 30 * time.Second
	idleTimeout     = 60 * time.Second
	shutdownTimeout = 10 * time.Second
	// Sesi login yang kedaluwarsa dan kunci idempotensi lama dihapus saat server mulai, lalu sekali sehari.
	cleanupInterval = 24 * time.Hour
)

type (
	Server struct {
		Cfg      *config.Config
		InfraM   manager.InfraManager
		RepoM    manager.RepositoryManager
		UseCaseM manager.UseCaseManager
		Engine   *fiber.App
		Host     string
		Log      *logrus.Logger
		logFile  *os.File
	}
)

// Run melayani request sampai menerima SIGINT (Ctrl+C) atau SIGTERM, lalu berhenti dengan rapi. Sinyal kedua
// menghentikan proses saat itu juga.
func (s *Server) Run() {
	s.initMiddlewares()
	s.initControllers()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop)

	s.Log.WithFields(logrus.Fields{
		"addr":              s.Host,
		"db":                s.Cfg.DbConfig.Name,
		"db_max_conns":      s.Cfg.DbConfig.MaxConns,
		"statement_timeout": s.Cfg.DbConfig.StatementTimeout.String(),
		"lock_timeout":      s.Cfg.DbConfig.LockTimeout.String(),
	}).Info("server mulai")
	go s.cleanup(ctx)
	err := s.Engine.Listen(s.Host, fiber.ListenConfig{GracefulContext: ctx, ShutdownTimeout: shutdownTimeout})
	if err != nil {
		s.Log.WithError(err).Error("server berhenti karena error")
	} else {
		s.Log.Info("server berhenti")
	}
	if cerr := s.InfraM.Conn().Close(); cerr != nil {
		s.Log.WithError(cerr).Error("gagal menutup koneksi database")
	}
	if cerr := s.logFile.Close(); cerr != nil {
		fmt.Fprintln(os.Stderr, "gagal menutup file log:", cerr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// cleanup menghapus sesi login yang kedaluwarsa dan kunci idempotensi yang lebih tua dari 30 hari, sampai ctx selesai
// (server berhenti). Kegagalan hanya dicatat di log: sesi yang tertinggal tidak bisa dipakai lagi, kunci lama tidak
// mengganggu, dan keduanya dihapus di putaran berikutnya.
func (s *Server) cleanup(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		n, err := s.UseCaseM.AuthUseCase().DeleteExpiredSessions()
		if err != nil {
			s.Log.WithError(err).Warn("gagal menghapus sesi login yang kedaluwarsa")
		} else if n > 0 {
			s.Log.WithField("deleted", n).Info("sesi login kedaluwarsa dihapus")
		}
		n, err = s.UseCaseM.IdempotencyUseCase().DeleteOldKeys()
		if err != nil {
			s.Log.WithError(err).Warn("gagal menghapus kunci idempotensi lama")
		} else if n > 0 {
			s.Log.WithField("deleted", n).Info("kunci idempotensi lama dihapus")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) initMiddlewares() {
	s.Engine.Use(middleware.Logging(s.Log))
	// Di dalam Logging, supaya request yang panic tetap tercatat sebagai 500 di log akses.
	s.Engine.Use(middleware.Recover())
}

func (s *Server) initControllers() {
	authenticate := middleware.Authenticate(s.Cfg.JWTConfig, s.RepoM.UserRepository())
	controller.NewHealthController(s.InfraM.Conn(), s.Engine).Route()
	controller.NewAuthController(s.Engine, s.UseCaseM.AuthUseCase(), authenticate).Route()
	controller.NewProductController(s.Engine, s.UseCaseM.ProductUseCase(), authenticate).Route()
	controller.NewTransactionController(s.Engine, s.UseCaseM.TransactionUseCase(), authenticate).Route()
	controller.NewUserController(s.Engine, s.UseCaseM.UserUseCase(), authenticate).Route()
	controller.NewReportController(s.Engine, s.UseCaseM.ReportUseCase(), authenticate).Route()
	controller.NewPreorderController(s.Engine, s.UseCaseM.PreorderUseCase(), authenticate).Route()
	controller.NewDeliveryController(s.Engine, s.UseCaseM.DeliveryUseCase(), authenticate).Route()
	controller.NewAuditController(s.Engine, s.UseCaseM.AuditUseCase(), authenticate).Route()
}

// NewServer Constructor
func NewServer() *Server {
	cfg, err := config.NewConfig()
	if err != nil {
		logrus.Fatalln(err.Error())
	}
	logger, logFile, err := newLogger(cfg.FileLogConfig)
	if err != nil {
		logrus.Fatalln(err.Error())
	}

	infraManager, err := manager.NewInfraManager(cfg)
	if err != nil {
		logrus.Fatalln(err.Error())
	}
	repoManager := manager.NewRepositoryManager(infraManager)
	useCaseManager := manager.NewUseCaseManager(repoManager, cfg.JWTConfig)

	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.NewErrorHandler(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	})
	return &Server{
		Cfg:      cfg,
		InfraM:   infraManager,
		RepoM:    repoManager,
		UseCaseM: useCaseManager,
		Engine:   app,
		Host:     fmt.Sprintf("%s:%s", cfg.ApiHost, cfg.ApiPort),
		Log:      logger,
		logFile:  logFile,
	}
}

// newLogger menulis ke file APP_FILE_PATH, satu baris key=value per kejadian, dengan waktu sampai milidetik.
func newLogger(cfg config.FileLogConfig) (*logrus.Logger, *os.File, error) {
	file, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return nil, nil, fmt.Errorf("buka file log: %w", err)
	}
	logger := logrus.New()
	logger.SetOutput(file)
	logger.SetLevel(cfg.LogLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})
	return logger, file, nil
}
