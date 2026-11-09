package manager

import (
	"github.com/EkoEdyP/point-of-sale/config"
	"github.com/EkoEdyP/point-of-sale/usecase"
)

type (
	UseCaseManager interface {
		AuthUseCase() usecase.AuthUseCase
		ProductUseCase() usecase.ProductUseCase
		TransactionUseCase() usecase.TransactionUseCase
		UserUseCase() usecase.UserUseCase
		ReportUseCase() usecase.ReportUseCase
		PreorderUseCase() usecase.PreorderUseCase
		DeliveryUseCase() usecase.DeliveryUseCase
		AuditUseCase() usecase.AuditUseCase
		IdempotencyUseCase() usecase.IdempotencyUseCase
	}
	useCaseManager struct {
		RepoM  RepositoryManager
		JWTCfg config.JWTConfig
	}
)

// AuthUseCase implement from UseCaseManager
func (u *useCaseManager) AuthUseCase() usecase.AuthUseCase {
	return usecase.NewAuthUseCase(u.RepoM.UserRepository(), u.RepoM.RefreshTokenRepository(), u.JWTCfg)
}

// ProductUseCase implement from UseCaseManager
func (u *useCaseManager) ProductUseCase() usecase.ProductUseCase {
	return usecase.NewProductUseCase(u.RepoM.ProductRepository(), u.RepoM.IdempotencyRepository())
}

// TransactionUseCase implement from UseCaseManager
func (u *useCaseManager) TransactionUseCase() usecase.TransactionUseCase {
	return usecase.NewTransactionUseCase(
		u.RepoM.TransactionRepository(),
		u.RepoM.ProductRepository(),
		u.RepoM.IdempotencyRepository(),
	)
}

// UserUseCase implement from UseCaseManager
func (u *useCaseManager) UserUseCase() usecase.UserUseCase {
	return usecase.NewUserUseCase(u.RepoM.UserRepository(), u.RepoM.RefreshTokenRepository())
}

// ReportUseCase implement from UseCaseManager
func (u *useCaseManager) ReportUseCase() usecase.ReportUseCase {
	return usecase.NewReportUseCase(u.RepoM.ReportRepository())
}

// PreorderUseCase implement from UseCaseManager
func (u *useCaseManager) PreorderUseCase() usecase.PreorderUseCase {
	return usecase.NewPreorderUseCase(
		u.RepoM.PreorderRepository(),
		u.RepoM.TransactionRepository(),
		u.RepoM.ProductRepository(),
		u.RepoM.IdempotencyRepository(),
	)
}

// DeliveryUseCase implement from UseCaseManager
func (u *useCaseManager) DeliveryUseCase() usecase.DeliveryUseCase {
	return usecase.NewDeliveryUseCase(u.RepoM.DeliveryRepository())
}

// AuditUseCase implement from UseCaseManager
func (u *useCaseManager) AuditUseCase() usecase.AuditUseCase {
	return usecase.NewAuditUseCase(u.RepoM.AuditRepository())
}

// IdempotencyUseCase implement from UseCaseManager
func (u *useCaseManager) IdempotencyUseCase() usecase.IdempotencyUseCase {
	return usecase.NewIdempotencyUseCase(u.RepoM.IdempotencyRepository())
}

// NewUseCaseManager Constructor
func NewUseCaseManager(repoM RepositoryManager, jwtCfg config.JWTConfig) UseCaseManager {
	return &useCaseManager{
		RepoM:  repoM,
		JWTCfg: jwtCfg,
	}
}
