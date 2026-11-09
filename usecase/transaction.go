package usecase

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/go-playground/validator/v10"
)

const maxQtyPerProduct = 10000

var errTransactionNotFound = &exception.GetError{Code: http.StatusNotFound, Message: "transaksi tidak ditemukan"}

// Penyerahan pre-order tidak bisa di-void: PO-nya tetap selesai, jadi stok, omzet, dan DP akan tidak cocok lagi.
var errPreorderTransaction = &exception.GetError{
	Code:    http.StatusConflict,
	Message: "transaksi dari pre-order tidak bisa dibatalkan",
}

type (
	TransactionUseCase interface {
		Create(cashierId string, request req.TransactionRequest) (res.TransactionResponse, error)
		List(actorId, role string, query req.TransactionListQuery) (res.TransactionListResponse, error)
		Get(actorId, role, id string) (res.TransactionResponse, error)
		Void(actorId, role, id string, request req.VoidRequest) (res.TransactionResponse, error)
	}
	transactionUseCase struct {
		transactions postgres.TransactionRepository
		products     postgres.ProductRepository
		keys         postgres.IdempotencyRepository
		validate     *validator.Validate
	}
)

// saleKeyPayload: isi yang menentukan "penjualan yang sama" untuk kunci idempotensi: barang (sudah digabung dan
// diurutkan) dan metode bayar. Uang yang diterima tidak ikut, karena kasir bisa mengetik ulang nominalnya saat
// mencoba lagi untuk penjualan yang sama.
type saleKeyPayload struct {
	Items         []itemQty `json:"items"`
	PaymentMethod string    `json:"payment_method"`
}

func (u *transactionUseCase) Create(cashierId string, request req.TransactionRequest) (res.TransactionResponse, error) {
	if err := u.validate.Struct(request); err != nil {
		return res.TransactionResponse{}, validationError(err)
	}

	lines := make([]itemQty, len(request.Items))
	for i, item := range request.Items {
		lines[i] = itemQty{ProductId: item.ProductId, Qty: item.Qty}
	}
	merged, err := mergeItems(lines)
	if err != nil {
		return res.TransactionResponse{}, err
	}
	key, err := requestKey(request.RequestKey, cashierId, actionSale,
		saleKeyPayload{Items: merged, PaymentMethod: request.PaymentMethod})
	if err != nil {
		return res.TransactionResponse{}, err
	}
	if dup, err := earlierRequest(u.keys, key); err != nil || dup != nil {
		if err != nil {
			return res.TransactionResponse{}, err
		}
		return u.replay(dup)
	}

	priced, subtotal, err := priceItems(u.products, merged, model.ProductStore)
	if err != nil {
		return res.TransactionResponse{}, err
	}
	items := make([]model.TransactionItem, 0, len(priced))
	for _, p := range priced {
		items = append(items, model.TransactionItem{
			ProductId:   p.product.Id,
			ProductName: p.product.Name,
			UnitPrice:   p.unitPrice,
			Qty:         p.qty,
			LineTotal:   p.lineTotal,
		})
	}

	total := subtotal // diskon ditunda, nilainya 0
	if err := checkExpectedTotal(request.ExpectedTotal, total); err != nil {
		return res.TransactionResponse{}, err
	}
	paid := request.Paid
	var change int64
	if request.PaymentMethod == model.PaymentCash {
		if paid < total {
			return res.TransactionResponse{}, unprocessable("pembayaran kurang dari total")
		}
		change = paid - total
	} else {
		paid = total
	}

	created, err := u.transactions.Create(model.Transaction{
		CashierId:     cashierId,
		Subtotal:      subtotal,
		Discount:      0,
		Total:         total,
		Paid:          paid,
		ChangeAmount:  change,
		PaymentMethod: request.PaymentMethod,
		Items:         items,
	}, key)
	if err != nil {
		var dup *postgres.DuplicateRequestError
		if errors.As(err, &dup) {
			return u.replay(dup)
		}
		return res.TransactionResponse{}, internalError(err)
	}

	// Ambil ulang supaya response lengkap (termasuk nama kasir) untuk cetak struk.
	full, err := u.transactions.FindById(created.Id)
	if err != nil {
		return res.TransactionResponse{}, internalError(err)
	}
	return toTransactionResponse(full, true), nil
}

// replay menjawab penjualan yang dikirim ulang dengan kunci idempotensi yang sudah tersimpan: struk penjualan yang
// sama kalau isinya sama, atau 409 kalau isinya berbeda (kasir mengubah keranjang setelah percobaan yang ternyata
// sudah tersimpan).
func (u *transactionUseCase) replay(dup *postgres.DuplicateRequestError) (res.TransactionResponse, error) {
	t, err := u.transactions.FindById(dup.TargetId)
	if err != nil {
		return res.TransactionResponse{}, internalError(err)
	}
	if !dup.Same {
		return res.TransactionResponse{}, duplicateRequest(
			"penjualan sebelumnya ternyata sudah tersimpan sebagai " + t.ReceiptNo +
				". Periksa Riwayat sebelum menjual lagi.",
		)
	}
	return toTransactionResponse(t, true), nil
}

// List: admin melihat semua transaksi, kasir hanya miliknya sendiri.
func (u *transactionUseCase) List(actorId, role string, query req.TransactionListQuery) (res.TransactionListResponse, error) {
	status := strings.TrimSpace(query.Status)
	if status != "" && status != model.StatusCompleted && status != model.StatusVoid {
		return res.TransactionListResponse{}, badRequest("status tidak valid (completed atau void)")
	}
	from, to, err := parseDateRange(strings.TrimSpace(query.From), strings.TrimSpace(query.To))
	if err != nil {
		return res.TransactionListResponse{}, err
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

	filter := postgres.TransactionFilter{
		Status: status,
		From:   from,
		To:     to,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
	if role != model.RoleAdmin {
		filter.CashierId = actorId
	}

	transactions, total, err := u.transactions.List(filter)
	if err != nil {
		return res.TransactionListResponse{}, internalError(err)
	}
	data := make([]res.TransactionResponse, 0, len(transactions))
	for _, t := range transactions {
		data = append(data, toTransactionResponse(t, false))
	}
	return res.TransactionListResponse{Data: data, Page: page, Limit: limit, Total: total}, nil
}

func (u *transactionUseCase) Get(actorId, role, id string) (res.TransactionResponse, error) {
	t, err := u.find(id)
	if err != nil {
		return res.TransactionResponse{}, err
	}
	// Transaksi kasir lain disamarkan sebagai "tidak ditemukan".
	if role != model.RoleAdmin && t.CashierId != actorId {
		return res.TransactionResponse{}, errTransactionNotFound
	}
	return toTransactionResponse(t, true), nil
}

// Void: admin boleh membatalkan transaksi mana pun, kasir hanya transaksinya sendiri.
// Alasan wajib diisi, dan siapa serta kapan tercatat.
func (u *transactionUseCase) Void(actorId, role, id string, request req.VoidRequest) (res.TransactionResponse, error) {
	request.Reason = strings.TrimSpace(request.Reason)
	if err := u.validate.Struct(request); err != nil {
		return res.TransactionResponse{}, validationError(err)
	}

	t, err := u.find(id)
	if err != nil {
		return res.TransactionResponse{}, err
	}
	if role != model.RoleAdmin && t.CashierId != actorId {
		return res.TransactionResponse{}, &exception.GetError{
			Code:    http.StatusForbidden,
			Message: "kasir hanya dapat membatalkan transaksinya sendiri",
		}
	}
	if t.Status == model.StatusVoid {
		return res.TransactionResponse{}, errAlreadyVoided
	}
	if t.PreorderId != nil {
		return res.TransactionResponse{}, errPreorderTransaction
	}

	if err := u.transactions.Void(id, actorId, request.Reason); err != nil {
		if errors.Is(err, postgres.ErrTransactionNotVoidable) {
			return res.TransactionResponse{}, errAlreadyVoided
		}
		return res.TransactionResponse{}, internalError(err)
	}

	updated, err := u.find(id)
	if err != nil {
		return res.TransactionResponse{}, err
	}
	return toTransactionResponse(updated, true), nil
}

var errAlreadyVoided = &exception.GetError{Code: http.StatusConflict, Message: "transaksi sudah dibatalkan"}

func (u *transactionUseCase) find(id string) (model.Transaction, error) {
	if !validUUID(id) {
		return model.Transaction{}, errTransactionNotFound
	}
	t, err := u.transactions.FindById(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Transaction{}, errTransactionNotFound
		}
		return model.Transaction{}, internalError(err)
	}
	return t, nil
}

// parseDateRange membaca YYYY-MM-DD (zona waktu server). "to" inklusif, jadi batas atasnya hari berikutnya.
func parseDateRange(from, to string) (*time.Time, *time.Time, error) {
	const layout = "2006-01-02"
	var f, t *time.Time
	if from != "" {
		d, err := time.ParseInLocation(layout, from, time.Local)
		if err != nil {
			return nil, nil, badRequest("from tidak valid, gunakan format YYYY-MM-DD")
		}
		f = &d
	}
	if to != "" {
		d, err := time.ParseInLocation(layout, to, time.Local)
		if err != nil {
			return nil, nil, badRequest("to tidak valid, gunakan format YYYY-MM-DD")
		}
		d = d.AddDate(0, 0, 1)
		t = &d
	}
	if f != nil && t != nil && !f.Before(*t) {
		return nil, nil, badRequest("from tidak boleh setelah to")
	}
	return f, t, nil
}

func toTransactionResponse(t model.Transaction, withItems bool) res.TransactionResponse {
	source := model.SourceStore
	if t.PreorderId != nil {
		source = model.SourcePreorder
	}
	response := res.TransactionResponse{
		Id:            t.Id,
		ReceiptNo:     t.ReceiptNo,
		CashierId:     t.CashierId,
		CashierName:   t.CashierName,
		Subtotal:      t.Subtotal,
		Discount:      t.Discount,
		ShippingFee:   t.ShippingFee,
		OtherFee:      t.OtherFee,
		OtherFeeNote:  t.OtherFeeNote,
		Total:         t.Total,
		Paid:          t.Paid,
		Change:        t.ChangeAmount,
		DownPayment:   t.DownPayment,
		PaymentMethod: t.PaymentMethod,
		Status:        t.Status,
		VoidedBy:      t.VoidedBy,
		VoidedByName:  t.VoidedByName,
		VoidedAt:      t.VoidedAt,
		VoidReason:    t.VoidReason,
		CreatedAt:     t.CreatedAt,
		Source:        source,
		PreorderId:    t.PreorderId,
		PreorderNo:    t.PreorderNo,
	}
	if withItems {
		response.Items = make([]res.TransactionItemResponse, 0, len(t.Items))
		for _, item := range t.Items {
			response.Items = append(response.Items, res.TransactionItemResponse{
				Id:          item.Id,
				ProductId:   item.ProductId,
				ProductName: item.ProductName,
				UnitPrice:   item.UnitPrice,
				Qty:         item.Qty,
				Discount:    item.Discount,
				LineTotal:   item.LineTotal,
			})
		}
	}
	return response
}

func badRequest(message string) error {
	return &exception.GetError{Code: http.StatusBadRequest, Message: message}
}

func unprocessable(message string) error {
	return &exception.GetError{Code: http.StatusUnprocessableEntity, Message: message}
}

// NewTransactionUseCase Constructor
func NewTransactionUseCase(
	transactions postgres.TransactionRepository,
	products postgres.ProductRepository,
	keys postgres.IdempotencyRepository,
) TransactionUseCase {
	return &transactionUseCase{
		transactions: transactions,
		products:     products,
		keys:         keys,
		validate:     newValidator(),
	}
}
