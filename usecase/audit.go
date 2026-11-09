package usecase

import (
	"slices"
	"strings"

	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
)

// auditEntities: jenis data yang dicatat riwayat perubahannya (audit_logs.entity).
var auditEntities = []string{"product", "preorder", "delivery", "user", "transaction", "setting"}

type (
	// AuditUseCase membaca riwayat perubahan data (hanya admin, dibatasi di controller).
	AuditUseCase interface {
		List(query req.AuditListQuery) (res.AuditListResponse, error)
	}
	auditUseCase struct {
		audits postgres.AuditRepository
	}
)

// List mengembalikan riwayat terbaru dulu.
func (u *auditUseCase) List(query req.AuditListQuery) (res.AuditListResponse, error) {
	entity := strings.TrimSpace(query.Entity)
	if entity != "" && !slices.Contains(auditEntities, entity) {
		return res.AuditListResponse{}, badRequest("entity tidak valid (" + strings.Join(auditEntities, ", ") + ")")
	}
	entityId := strings.TrimSpace(query.EntityId)
	if len(entityId) > 64 {
		return res.AuditListResponse{}, badRequest("entity_id tidak valid")
	}
	actorId := strings.TrimSpace(query.ActorId)
	if actorId != "" && !validUUID(actorId) {
		return res.AuditListResponse{}, badRequest("actor_id tidak valid")
	}
	from, to, err := parseDateRange(strings.TrimSpace(query.From), strings.TrimSpace(query.To))
	if err != nil {
		return res.AuditListResponse{}, err
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	entries, total, err := u.audits.List(postgres.AuditFilter{
		Entity:   entity,
		EntityId: entityId,
		ActorId:  actorId,
		From:     from,
		To:       to,
		Limit:    limit,
		Offset:   (page - 1) * limit,
	})
	if err != nil {
		return res.AuditListResponse{}, internalError(err)
	}
	data := make([]res.AuditEntryResponse, 0, len(entries))
	for _, e := range entries {
		data = append(data, res.AuditEntryResponse{
			Id:          e.Id,
			CreatedAt:   e.CreatedAt,
			ActorId:     e.ActorId,
			ActorName:   e.ActorName,
			Action:      e.Action,
			Entity:      e.Entity,
			EntityId:    e.EntityId,
			EntityLabel: e.EntityLabel,
			Changes:     e.Changes,
			Note:        e.Note,
		})
	}
	return res.AuditListResponse{Data: data, Page: page, Limit: limit, Total: total}, nil
}

// NewAuditUseCase Constructor
func NewAuditUseCase(audits postgres.AuditRepository) AuditUseCase {
	return &auditUseCase{audits: audits}
}
