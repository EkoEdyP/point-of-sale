package usecase

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/go-playground/validator/v10"
)

const maxDeliveryFee = 100_000_000

var (
	errDeliveryNotFound = &exception.GetError{Code: http.StatusNotFound, Message: "pengiriman tidak ditemukan"}
	errDeliveryClosed   = &exception.GetError{Code: http.StatusConflict, Message: "status pengiriman tidak mengizinkan tindakan ini"}
)

type (
	// Pengiriman hanya untuk pre-order "diantar" (dibuat otomatis bersama pre-ordernya).
	// Penjualan toko tidak pernah diantar, jadi tidak ada pembuatan pengiriman dari transaksi.
	DeliveryUseCase interface {
		List(query req.DeliveryListQuery) (res.DeliveryListResponse, error)
		Get(id string) (res.DeliveryResponse, error)
		Summary() (res.DeliverySummaryResponse, error)
		Update(actorId, id string, request req.DeliveryUpdateRequest) (res.DeliveryResponse, error)
		Dispatch(actorId, id string, request req.DeliveryDispatchRequest) (res.DeliveryResponse, error)
		Fail(actorId, id string, request req.DeliveryReasonRequest) (res.DeliveryResponse, error)
		Cancel(actorId, role, id string, request req.DeliveryReasonRequest) (res.DeliveryResponse, error)
	}
	deliveryUseCase struct {
		deliveries postgres.DeliveryRepository
		validate   *validator.Validate
	}
)

// List: "active" (bawaan) berisi pengiriman yang masih perlu ditangani, termasuk yang gagal supaya tidak hilang.
func (u *deliveryUseCase) List(query req.DeliveryListQuery) (res.DeliveryListResponse, error) {
	var statuses []string
	sortByDue := false
	switch strings.TrimSpace(query.Status) {
	case "", "active":
		statuses = []string{model.DeliveryPending, model.DeliveryOnTheWay, model.DeliveryFailed}
		sortByDue = true
	case "done":
		statuses = []string{model.DeliveryDelivered, model.DeliveryCancelled}
	case "all":
		statuses = nil
	default:
		return res.DeliveryListResponse{}, badRequest("status tidak valid (active, done, atau all)")
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

	deliveries, total, err := u.deliveries.List(postgres.DeliveryFilter{
		Statuses:  statuses,
		SortByDue: sortByDue,
		Limit:     limit,
		Offset:    (page - 1) * limit,
	})
	if err != nil {
		return res.DeliveryListResponse{}, internalError(err)
	}

	data := make([]res.DeliveryResponse, 0, len(deliveries))
	for _, d := range deliveries {
		data = append(data, toDeliveryResponse(d, false))
	}
	return res.DeliveryListResponse{Data: data, Page: page, Limit: limit, Total: total}, nil
}

func (u *deliveryUseCase) Get(id string) (res.DeliveryResponse, error) {
	d, err := u.find(id)
	if err != nil {
		return res.DeliveryResponse{}, err
	}
	return toDeliveryResponse(d, true), nil
}

func (u *deliveryUseCase) Summary() (res.DeliverySummaryResponse, error) {
	s, err := u.deliveries.Summary()
	if err != nil {
		return res.DeliverySummaryResponse{}, internalError(err)
	}
	return res.DeliverySummaryResponse{Overdue: s.Overdue, Today: s.Today, Tomorrow: s.Tomorrow}, nil
}

// Update mengubah data pengiriman. Pengiriman yang gagal dijadwalkan ulang dengan mengirim tanggal baru,
// dan statusnya kembali menjadi menunggu. Pengiriman pre-order ikut mengubah pre-order-nya (pemesan, alamat,
// jadwal, catatan), jadi tanggal baru juga memindah hari H-nya dan dicek kuotanya.
func (u *deliveryUseCase) Update(actorId, id string, request req.DeliveryUpdateRequest) (res.DeliveryResponse, error) {
	if request.CustomerName == nil && request.CustomerPhone == nil && request.Address == nil &&
		request.ScheduledDate == nil && request.ScheduledTime == nil && request.Courier == nil &&
		request.Fee == nil && request.Notes == nil {
		return res.DeliveryResponse{}, badRequest("tidak ada field yang diubah")
	}

	d, err := u.find(id)
	if err != nil {
		return res.DeliveryResponse{}, err
	}
	if d.Status != model.DeliveryPending && d.Status != model.DeliveryOnTheWay && d.Status != model.DeliveryFailed {
		return res.DeliveryResponse{}, errDeliveryClosed
	}
	// Perubahan digabungkan dengan d, dan repo hanya menyimpannya selama barisnya masih di versi d.Version.
	if err := checkVersion(request.Version, d.Version, "pengiriman"); err != nil {
		return res.DeliveryResponse{}, err
	}

	if request.CustomerName != nil {
		v := strings.TrimSpace(*request.CustomerName)
		if v == "" || utf8.RuneCountInString(v) > 100 {
			return res.DeliveryResponse{}, badRequest("customer_name tidak valid (1-100 karakter)")
		}
		d.CustomerName = v
	}
	if request.CustomerPhone != nil {
		d.CustomerPhone, err = cleanOptional(request.CustomerPhone, 30, "customer_phone")
		if err != nil {
			return res.DeliveryResponse{}, err
		}
	}
	if request.Address != nil {
		v := strings.TrimSpace(*request.Address)
		if v == "" || utf8.RuneCountInString(v) > 300 {
			return res.DeliveryResponse{}, badRequest("address tidak valid (1-300 karakter)")
		}
		d.Address = v
	}
	if request.ScheduledDate != nil {
		// Menjadwalkan ulang pengiriman yang gagal selalu butuh tanggal hari ini atau sesudahnya.
		current := d.ScheduledDate
		if d.Status == model.DeliveryFailed {
			current = ""
		}
		d.ScheduledDate, err = checkScheduleDate(*request.ScheduledDate, current, "scheduled_date", "tanggal kirim")
		if err != nil {
			return res.DeliveryResponse{}, err
		}
	}
	if request.ScheduledTime != nil {
		d.ScheduledTime, err = cleanTime(request.ScheduledTime)
		if err != nil {
			return res.DeliveryResponse{}, err
		}
	}
	if request.Courier != nil {
		d.Courier, err = cleanOptional(request.Courier, 100, "courier")
		if err != nil {
			return res.DeliveryResponse{}, err
		}
	}
	if request.Fee != nil {
		if *request.Fee < 0 || *request.Fee > maxDeliveryFee {
			return res.DeliveryResponse{}, badRequest("fee tidak valid")
		}
		// Ongkir pre-order ikut total tagihan pelanggan, jadi hanya diubah dari pre-ordernya.
		if d.PreorderId != nil && *request.Fee != d.Fee {
			return res.DeliveryResponse{}, unprocessable("ongkir pre-order diubah dari detail pre-order (Ubah biaya)")
		}
		d.Fee = *request.Fee
	}
	if request.Notes != nil {
		d.Notes, err = cleanOptional(request.Notes, 500, "notes")
		if err != nil {
			return res.DeliveryResponse{}, err
		}
	}

	if d.Status == model.DeliveryFailed {
		if request.ScheduledDate == nil {
			return res.DeliveryResponse{}, badRequest("jadwal baru wajib diisi untuk menjadwalkan ulang pengiriman yang gagal")
		}
		d.Status = model.DeliveryPending
		d.StatusNote = nil
	}

	if err := u.deliveries.Update(id, d, postgres.Audit{ActorId: actorId}); err != nil {
		var quota *postgres.QuotaExceededError
		switch {
		case errors.Is(err, postgres.ErrDeliveryClosed):
			return res.DeliveryResponse{}, errDeliveryClosed
		case errors.Is(err, postgres.ErrStale):
			return res.DeliveryResponse{}, staleError("pengiriman")
		case errors.As(err, &quota):
			// Tanggal kirim pengiriman pre-order = hari H-nya, jadi kuotanya sama dengan memindah hari H.
			return res.DeliveryResponse{}, quotaExceeded(quota)
		}
		return res.DeliveryResponse{}, internalError(err)
	}
	return u.Get(id)
}

// Dispatch: kurir berangkat. Nama kurir bisa diisi sekarang atau sudah diisi sebelumnya.
func (u *deliveryUseCase) Dispatch(
	actorId, id string, request req.DeliveryDispatchRequest,
) (res.DeliveryResponse, error) {
	d, err := u.find(id)
	if err != nil {
		return res.DeliveryResponse{}, err
	}
	if d.Status != model.DeliveryPending {
		return res.DeliveryResponse{}, &exception.GetError{
			Code:    http.StatusConflict,
			Message: "hanya pengiriman yang menunggu yang bisa diberangkatkan",
		}
	}
	courier, err := cleanOptional(request.Courier, 100, "courier")
	if err != nil {
		return res.DeliveryResponse{}, err
	}

	if err := u.deliveries.Dispatch(id, courier, postgres.Audit{ActorId: actorId}); err != nil {
		if errors.Is(err, postgres.ErrDeliveryClosed) {
			return res.DeliveryResponse{}, errDeliveryClosed
		}
		return res.DeliveryResponse{}, internalError(err)
	}
	return u.Get(id)
}

func (u *deliveryUseCase) Fail(actorId, id string, request req.DeliveryReasonRequest) (res.DeliveryResponse, error) {
	request.Reason = strings.TrimSpace(request.Reason)
	if err := u.validate.Struct(request); err != nil {
		return res.DeliveryResponse{}, validationError(err)
	}
	d, err := u.find(id)
	if err != nil {
		return res.DeliveryResponse{}, err
	}
	if d.Status != model.DeliveryPending && d.Status != model.DeliveryOnTheWay {
		return res.DeliveryResponse{}, errDeliveryClosed
	}

	if err := u.deliveries.Fail(id, request.Reason, postgres.Audit{ActorId: actorId, Note: request.Reason}); err != nil {
		if errors.Is(err, postgres.ErrDeliveryClosed) {
			return res.DeliveryResponse{}, errDeliveryClosed
		}
		return res.DeliveryResponse{}, internalError(err)
	}
	return u.Get(id)
}

// Cancel: admin boleh membatalkan pengiriman mana pun, kasir hanya yang dibuatnya sendiri. Alasan wajib.
func (u *deliveryUseCase) Cancel(actorId, role, id string, request req.DeliveryReasonRequest) (res.DeliveryResponse, error) {
	request.Reason = strings.TrimSpace(request.Reason)
	if err := u.validate.Struct(request); err != nil {
		return res.DeliveryResponse{}, validationError(err)
	}
	d, err := u.find(id)
	if err != nil {
		return res.DeliveryResponse{}, err
	}
	if role != model.RoleAdmin && d.CreatedBy != actorId {
		return res.DeliveryResponse{}, &exception.GetError{
			Code:    http.StatusForbidden,
			Message: "kasir hanya dapat membatalkan pengiriman buatannya sendiri",
		}
	}
	if d.Status != model.DeliveryPending && d.Status != model.DeliveryOnTheWay && d.Status != model.DeliveryFailed {
		return res.DeliveryResponse{}, errDeliveryClosed
	}

	if err := u.deliveries.Cancel(id, request.Reason, postgres.Audit{ActorId: actorId, Note: request.Reason}); err != nil {
		if errors.Is(err, postgres.ErrDeliveryClosed) {
			return res.DeliveryResponse{}, errDeliveryClosed
		}
		return res.DeliveryResponse{}, internalError(err)
	}
	return u.Get(id)
}

func (u *deliveryUseCase) find(id string) (model.Delivery, error) {
	if !validUUID(id) {
		return model.Delivery{}, errDeliveryNotFound
	}
	d, err := u.deliveries.FindById(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Delivery{}, errDeliveryNotFound
		}
		return model.Delivery{}, internalError(err)
	}
	return d, nil
}

func toDeliveryResponse(d model.Delivery, withItems bool) res.DeliveryResponse {
	sourceType, sourceId := "transaction", ""
	if d.PreorderId != nil {
		sourceType, sourceId = "preorder", *d.PreorderId
	} else if d.TransactionId != nil {
		sourceId = *d.TransactionId
	}

	response := res.DeliveryResponse{
		Id:             d.Id,
		DeliveryNo:     d.DeliveryNo,
		SourceType:     sourceType,
		SourceId:       sourceId,
		SourceNo:       d.SourceNo,
		CustomerName:   d.CustomerName,
		CustomerPhone:  d.CustomerPhone,
		Address:        d.Address,
		ScheduledDate:  d.ScheduledDate,
		ScheduledTime:  d.ScheduledTime,
		Courier:        d.Courier,
		Fee:            d.Fee,
		Notes:          d.Notes,
		Status:         d.Status,
		StatusNote:     d.StatusNote,
		DispatchedAt:   d.DispatchedAt,
		DeliveredAt:    d.DeliveredAt,
		ReceivedBy:     d.ReceivedBy,
		AmountDue:      d.AmountDue,
		Subtotal:       d.Subtotal,
		OtherFee:       d.OtherFee,
		OtherFeeNote:   d.OtherFeeNote,
		Total:          d.Total,
		DownPayment:    d.DownPayment,
		PreorderStatus: d.PreorderStatus,
		CreatedById:    d.CreatedBy,
		CreatedByName:  d.CreatedByName,
		CreatedAt:      d.CreatedAt,
		Version:        d.Version,
	}
	if withItems {
		response.Items = make([]res.DeliveryItemResponse, 0, len(d.Items))
		for _, item := range d.Items {
			response.Items = append(response.Items, res.DeliveryItemResponse{
				ProductName: item.ProductName, Qty: item.Qty, UnitPrice: item.UnitPrice, LineTotal: item.LineTotal,
			})
		}
	}
	return response
}

// NewDeliveryUseCase Constructor
func NewDeliveryUseCase(deliveries postgres.DeliveryRepository) DeliveryUseCase {
	return &deliveryUseCase{
		deliveries: deliveries,
		validate:   newValidator(),
	}
}
