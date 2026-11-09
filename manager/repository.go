package manager

import "github.com/EkoEdyP/point-of-sale/repository/postgres"

type (
	RepositoryManager interface {
		UserRepository() postgres.UserRepository
		RefreshTokenRepository() postgres.RefreshTokenRepository
		ProductRepository() postgres.ProductRepository
		TransactionRepository() postgres.TransactionRepository
		ReportRepository() postgres.ReportRepository
		PreorderRepository() postgres.PreorderRepository
		DeliveryRepository() postgres.DeliveryRepository
		AuditRepository() postgres.AuditRepository
		IdempotencyRepository() postgres.IdempotencyRepository
	}
	repositoryManager struct {
		InfraM InfraManager
	}
)

// UserRepository implement from RepositoryManager
func (r *repositoryManager) UserRepository() postgres.UserRepository {
	return postgres.NewUserRepository(r.InfraM.Conn())
}

// RefreshTokenRepository implement from RepositoryManager
func (r *repositoryManager) RefreshTokenRepository() postgres.RefreshTokenRepository {
	return postgres.NewRefreshTokenRepository(r.InfraM.Conn())
}

// ProductRepository implement from RepositoryManager
func (r *repositoryManager) ProductRepository() postgres.ProductRepository {
	return postgres.NewProductRepository(r.InfraM.Conn())
}

// TransactionRepository implement from RepositoryManager
func (r *repositoryManager) TransactionRepository() postgres.TransactionRepository {
	return postgres.NewTransactionRepository(r.InfraM.Conn())
}

// ReportRepository implement from RepositoryManager
func (r *repositoryManager) ReportRepository() postgres.ReportRepository {
	return postgres.NewReportRepository(r.InfraM.Conn())
}

// PreorderRepository implement from RepositoryManager
func (r *repositoryManager) PreorderRepository() postgres.PreorderRepository {
	return postgres.NewPreorderRepository(r.InfraM.Conn())
}

// DeliveryRepository implement from RepositoryManager
func (r *repositoryManager) DeliveryRepository() postgres.DeliveryRepository {
	return postgres.NewDeliveryRepository(r.InfraM.Conn())
}

// AuditRepository implement from RepositoryManager
func (r *repositoryManager) AuditRepository() postgres.AuditRepository {
	return postgres.NewAuditRepository(r.InfraM.Conn())
}

// IdempotencyRepository implement from RepositoryManager
func (r *repositoryManager) IdempotencyRepository() postgres.IdempotencyRepository {
	return postgres.NewIdempotencyRepository(r.InfraM.Conn())
}

// NewRepositoryManager Constructor
func NewRepositoryManager(infraM InfraManager) RepositoryManager {
	return &repositoryManager{
		InfraM: infraM,
	}
}
