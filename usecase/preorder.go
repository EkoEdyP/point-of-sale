package usecase

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/go-playground/validator/v10"
)

const dateLayout = "2006-01-02"

var (
	errPreorderNotFound = &exception.GetError{Code: http.StatusNotFound, Message: "pre-order tidak ditemukan"}
	errPreorderClosed   = &exception.GetError{Code: http.StatusConflict, Message: "pre-order sudah selesai atau dibatalkan"}
)

type (
	PreorderUseCase interface {
		Create(actorId string, request req.PreorderRequest) (res.PreorderResponse, error)
		List(query req.PreorderListQuery) (res.PreorderListResponse, error)
		Get(id string) (res.PreorderResponse, error)
		Summary() (res.PreorderSummaryResponse, error)
		Production(dueDate string) (res.PreorderProductionResponse, error)
		AddPayment(actorId, id string, request req.PreorderPaymentRequest) (res.PreorderResponse, error)
		Update(actorId, id string, request req.PreorderUpdateRequest) (res.PreorderResponse, error)
		MarkReady(actorId, id string) (res.PreorderResponse, error)
		Complete(actorId, id string, request req.PreorderCompleteRequest) (res.TransactionResponse, error)
		Cancel(actorId, role, id string, request req.PreorderCancelRequest) (res.PreorderResponse, error)
		// Quota: kuota hari H. excludeId (opsional): pre-order yang tidak ikut dihitung, untuk pindah jadwal.
		Quota(dueDate, excludeId string) (res.PreorderQuotaResponse, error)
		SetDailyLimit(actorId string, request req.PreorderDailyLimitRequest) (res.PreorderDailyLimitResponse, error)
	}
	preorderUseCase struct {
		preorders    postgres.PreorderRepository
		transactions postgres.TransactionRepository
		products     postgres.ProductRepository
		keys         postgres.IdempotencyRepository
		validate     *validator.Validate
	}
)

func (u *preorderUseCase) Create(actorId string, request req.PreorderRequest) (res.PreorderResponse, error) {
	request.CustomerName = strings.TrimSpace(request.CustomerName)
	if err := u.validate.Struct(request); err != nil {
		return res.PreorderResponse{}, validationError(err)
	}

	// Kunci idempotensi: pre-order yang sama = isi form yang sama, tanpa total di layar (bisa berubah kalau harga
	// dimuat ulang, padahal pesanannya sama).
	payload := request
	payload.ExpectedTotal = nil
	key, err := requestKey(request.RequestKey, actorId, actionPreorderCreate, payload)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	if dup, err := earlierRequest(u.keys, key); err != nil || dup != nil {
		if err != nil {
			return res.PreorderResponse{}, err
		}
		return u.replayCreate(dup)
	}

	phone, err := cleanOptional(request.CustomerPhone, 30, "customer_phone")
	if err != nil {
		return res.PreorderResponse{}, err
	}
	address, err := cleanOptional(request.Address, 300, "address")
	if err != nil {
		return res.PreorderResponse{}, err
	}
	notes, err := cleanOptional(request.Notes, 500, "notes")
	if err != nil {
		return res.PreorderResponse{}, err
	}
	dueTime, err := cleanTime(request.DueTime)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	dueDate, err := checkScheduleDate(request.DueDate, "", "due_date", "tanggal jatuh tempo")
	if err != nil {
		return res.PreorderResponse{}, err
	}

	if request.Fulfillment == model.FulfillmentDelivery {
		if address == nil {
			return res.PreorderResponse{}, badRequest("alamat pengantaran wajib diisi")
		}
	} else {
		address = nil
	}

	// Harga dikunci dari database saat pre-order dibuat (harga grosir kalau jumlahnya mencapai batas produk itu).
	// Perubahan harga produk sesudahnya tidak mengubah pre-order ini.
	lines := make([]itemQty, len(request.Items))
	for i, item := range request.Items {
		lines[i] = itemQty{ProductId: item.ProductId, Qty: item.Qty}
	}
	merged, err := mergeItems(lines)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	priced, subtotal, err := priceItems(u.products, merged, model.ProductPreorder)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	items := make([]model.PreorderItem, 0, len(priced))
	for _, p := range priced {
		items = append(items, model.PreorderItem{
			ProductId:   p.product.Id,
			ProductName: p.product.Name,
			UnitPrice:   p.unitPrice,
			Qty:         p.qty,
			LineTotal:   p.lineTotal,
		})
	}

	otherFeeNote, err := cleanFees(request.Fulfillment, request.ShippingFee, request.OtherFee, request.OtherFeeNote)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	total := subtotal + request.ShippingFee + request.OtherFee
	if err := checkExpectedTotal(request.ExpectedTotal, total); err != nil {
		return res.PreorderResponse{}, err
	}

	var downPayment *model.PreorderPayment
	if request.DownPayment > 0 {
		if request.DownPaymentMethod == "" {
			return res.PreorderResponse{}, badRequest("metode DP wajib diisi")
		}
		if request.DownPayment > total {
			return res.PreorderResponse{}, unprocessable("DP tidak boleh melebihi total pesanan")
		}
		downPayment = &model.PreorderPayment{
			Amount:    request.DownPayment,
			Method:    request.DownPaymentMethod,
			CreatedBy: actorId,
		}
	}

	id, err := u.preorders.Create(model.Preorder{
		CustomerName:  request.CustomerName,
		CustomerPhone: phone,
		DueDate:       dueDate,
		DueTime:       dueTime,
		Fulfillment:   request.Fulfillment,
		Address:       address,
		Notes:         notes,
		Subtotal:      subtotal,
		ShippingFee:   request.ShippingFee,
		OtherFee:      request.OtherFee,
		OtherFeeNote:  otherFeeNote,
		Total:         total,
		CreatedBy:     actorId,
		Items:         items,
	}, downPayment, key)
	if err != nil {
		var quota *postgres.QuotaExceededError
		if errors.As(err, &quota) {
			return res.PreorderResponse{}, quotaExceeded(quota)
		}
		var dup *postgres.DuplicateRequestError
		if errors.As(err, &dup) {
			return u.replayCreate(dup)
		}
		return res.PreorderResponse{}, internalError(err)
	}
	return u.Get(id)
}

// replayCreate menjawab pre-order yang dikirim ulang dengan kunci idempotensi yang sudah tersimpan.
func (u *preorderUseCase) replayCreate(dup *postgres.DuplicateRequestError) (res.PreorderResponse, error) {
	p, err := u.Get(dup.TargetId)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	if !dup.Same {
		return res.PreorderResponse{}, duplicateRequest(
			"pre-order sebelumnya ternyata sudah tersimpan sebagai " + p.OrderNo +
				". Periksa daftar pre-order sebelum menyimpan lagi.",
		)
	}
	return p, nil
}

// List: semua pengguna melihat semua pre-order, karena yang menerima pesanan belum tentu yang menyerahkannya.
func (u *preorderUseCase) List(query req.PreorderListQuery) (res.PreorderListResponse, error) {
	var statuses []string
	sortByDue := false
	switch strings.TrimSpace(query.Status) {
	case "", "open":
		statuses = []string{model.PreorderOrdered, model.PreorderReady}
		sortByDue = true
	case "completed":
		statuses = []string{model.PreorderCompleted}
	case "cancelled":
		statuses = []string{model.PreorderCancelled}
	case "all":
		statuses = nil
	default:
		return res.PreorderListResponse{}, badRequest("status tidak valid (open, completed, cancelled, atau all)")
	}

	// Filter hari H opsional. Tanggal lampau boleh, misalnya untuk melihat yang terlambat atau yang sudah selesai.
	dueDate := strings.TrimSpace(query.DueDate)
	if dueDate != "" {
		d, err := time.Parse(dateLayout, dueDate)
		// time.Parse menerima tahun 0000, tapi Postgres tidak punya tahun 0 (kueri gagal jadi 500).
		if err != nil || d.Year() < 1 {
			return res.PreorderListResponse{}, badRequest("due_date tidak valid, gunakan format YYYY-MM-DD")
		}
		dueDate = d.Format(dateLayout)
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

	preorders, total, err := u.preorders.List(postgres.PreorderFilter{
		Statuses:  statuses,
		DueDate:   dueDate,
		SortByDue: sortByDue,
		Limit:     limit,
		Offset:    (page - 1) * limit,
	})
	if err != nil {
		return res.PreorderListResponse{}, internalError(err)
	}

	data := make([]res.PreorderResponse, 0, len(preorders))
	for _, p := range preorders {
		data = append(data, toPreorderResponse(p, false))
	}
	return res.PreorderListResponse{Data: data, Page: page, Limit: limit, Total: total}, nil
}

func (u *preorderUseCase) Get(id string) (res.PreorderResponse, error) {
	p, err := u.find(id)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	return toPreorderResponse(p, true), nil
}

func (u *preorderUseCase) Summary() (res.PreorderSummaryResponse, error) {
	s, err := u.preorders.Summary()
	if err != nil {
		return res.PreorderSummaryResponse{}, internalError(err)
	}
	return res.PreorderSummaryResponse{
		Overdue:          s.Overdue,
		Today:            s.Today,
		Tomorrow:         s.Tomorrow,
		DayAfterTomorrow: s.DayAfterTomorrow,
	}, nil
}

// Production menghitung kebutuhan produksi dari pre-order yang belum diserahkan (dipesan atau siap).
// Stok siap tiap produk dipakai untuk hari H paling awal lebih dulu, termasuk yang terlambat:
// "siap" pada suatu hari adalah sisa stok setelah hari-hari sebelumnya terpenuhi.
// dueDate (opsional) hanya menyaring hari yang dikirim; pembagian stok tetap memakai semua hari.
func (u *preorderUseCase) Production(dueDate string) (res.PreorderProductionResponse, error) {
	dueDate = strings.TrimSpace(dueDate)
	if dueDate != "" {
		d, err := time.Parse(dateLayout, dueDate)
		if err != nil || d.Year() < 1 {
			return res.PreorderProductionResponse{}, badRequest("due_date tidak valid, gunakan format YYYY-MM-DD")
		}
		dueDate = d.Format(dateLayout)
	}

	needs, err := u.preorders.ProductionNeeds()
	if err != nil {
		return res.PreorderProductionResponse{}, internalError(err)
	}

	response := res.PreorderProductionResponse{
		Days:     []res.ProductionDayResponse{},
		Products: []res.ProductionProductResponse{},
	}
	available := make(map[string]int)          // sisa stok siap per produk
	totals := make(map[string]int, len(needs)) // posisi produk di response.Products
	for _, n := range needs {                  // urut hari H, jadi hari paling awal mendapat stok lebih dulu
		pos, seen := totals[n.ProductId]
		if !seen {
			available[n.ProductId] = max(n.Stock, 0)
			pos = len(response.Products)
			totals[n.ProductId] = pos
			response.Products = append(response.Products, res.ProductionProductResponse{
				ProductId:   n.ProductId,
				ProductName: n.ProductName,
				Stock:       n.Stock,
			})
		}
		ready := min(n.Needed, available[n.ProductId])
		available[n.ProductId] -= ready
		short := n.Needed - ready
		response.Products[pos].Needed += n.Needed
		response.Products[pos].Short += short

		if dueDate != "" && n.DueDate != dueDate {
			continue
		}
		if last := len(response.Days) - 1; last < 0 || response.Days[last].DueDate != n.DueDate {
			response.Days = append(response.Days, res.ProductionDayResponse{
				DueDate: n.DueDate,
				Items:   []res.ProductionItemResponse{},
			})
		}
		day := &response.Days[len(response.Days)-1]
		day.Items = append(day.Items, res.ProductionItemResponse{
			ProductId:   n.ProductId,
			ProductName: n.ProductName,
			Needed:      n.Needed,
			Ready:       ready,
			Short:       short,
			Orders:      n.Orders,
		})
	}

	sort.SliceStable(response.Products, func(i, j int) bool {
		return strings.ToLower(response.Products[i].ProductName) < strings.ToLower(response.Products[j].ProductName)
	})

	// Kuota per hari H, supaya terlihat seberapa penuh tiap hari.
	if response.DailyLimit, err = u.preorders.DailyLimit(); err != nil {
		return res.PreorderProductionResponse{}, internalError(err)
	}
	dates := make([]string, len(response.Days))
	for i, day := range response.Days {
		dates[i] = day.DueDate
	}
	used, err := u.preorders.QuotaUsed(dates, "")
	if err != nil {
		return res.PreorderProductionResponse{}, internalError(err)
	}
	for i := range response.Days {
		response.Days[i].QuotaUsed = used[response.Days[i].DueDate]
	}
	return response, nil
}

func (u *preorderUseCase) AddPayment(actorId, id string, request req.PreorderPaymentRequest) (res.PreorderResponse, error) {
	if err := u.validate.Struct(request); err != nil {
		return res.PreorderResponse{}, validationError(err)
	}
	if !validUUID(id) {
		return res.PreorderResponse{}, errPreorderNotFound
	}
	key, err := requestKey(request.RequestKey, actorId, actionPreorderPayment, struct {
		PreorderId string `json:"preorder_id"`
		Amount     int64  `json:"amount"`
		Method     string `json:"method"`
	}{strings.ToLower(id), request.Amount, request.Method})
	if err != nil {
		return res.PreorderResponse{}, err
	}
	if dup, err := earlierRequest(u.keys, key); err != nil || dup != nil {
		if err != nil {
			return res.PreorderResponse{}, err
		}
		return u.replayPayment(dup)
	}

	p, err := u.find(id)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	if !isOpen(p) {
		return res.PreorderResponse{}, errPreorderClosed
	}

	err = u.preorders.AddPayment(id, model.PreorderPayment{
		Amount:    request.Amount,
		Method:    request.Method,
		CreatedBy: actorId,
	}, key)
	if err != nil {
		var dup *postgres.DuplicateRequestError
		switch {
		case errors.Is(err, postgres.ErrPreorderClosed):
			return res.PreorderResponse{}, errPreorderClosed
		case errors.Is(err, postgres.ErrPreorderOverpaid):
			return res.PreorderResponse{}, unprocessable("total DP tidak boleh melebihi total pesanan")
		case errors.Is(err, sql.ErrNoRows):
			return res.PreorderResponse{}, errPreorderNotFound
		case errors.As(err, &dup):
			return u.replayPayment(dup)
		}
		return res.PreorderResponse{}, internalError(err)
	}
	return u.Get(id)
}

// replayPayment menjawab DP yang dikirim ulang dengan kunci idempotensi yang sudah tersimpan: pre-order-nya apa
// adanya (DP itu sudah tercatat sekali), atau 409 kalau isinya berbeda.
func (u *preorderUseCase) replayPayment(dup *postgres.DuplicateRequestError) (res.PreorderResponse, error) {
	if !dup.Same {
		return res.PreorderResponse{}, duplicateRequest(
			"DP sebelumnya ternyata sudah tercatat. Periksa detail pre-order sebelum menambah DP lagi.",
		)
	}
	return u.Get(dup.TargetId)
}

// Update mengubah data pelanggan (nama, No. HP, alamat), jadwal, catatan, ongkir, dan biaya lain. Item tidak bisa
// diubah: batalkan lalu buat pre-order baru.
func (u *preorderUseCase) Update(actorId, id string, request req.PreorderUpdateRequest) (res.PreorderResponse, error) {
	if request.CustomerName == nil && request.CustomerPhone == nil && request.Address == nil &&
		request.DueDate == nil && request.DueTime == nil && request.Notes == nil &&
		request.ShippingFee == nil && request.OtherFee == nil && request.OtherFeeNote == nil {
		return res.PreorderResponse{}, badRequest("tidak ada field yang diubah")
	}
	p, err := u.find(id)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	if !isOpen(p) {
		return res.PreorderResponse{}, errPreorderClosed
	}
	if err := checkVersion(request.Version, p.Version, "pre-order"); err != nil {
		return res.PreorderResponse{}, err
	}

	customerName := p.CustomerName
	if request.CustomerName != nil {
		customerName = strings.TrimSpace(*request.CustomerName)
		if customerName == "" || utf8.RuneCountInString(customerName) > 100 {
			return res.PreorderResponse{}, badRequest("customer_name tidak valid (1-100 karakter)")
		}
	}
	customerPhone := p.CustomerPhone
	if request.CustomerPhone != nil {
		customerPhone, err = cleanOptional(request.CustomerPhone, 30, "customer_phone")
		if err != nil {
			return res.PreorderResponse{}, err
		}
	}
	address := p.Address
	if request.Address != nil {
		if p.Fulfillment != model.FulfillmentDelivery {
			return res.PreorderResponse{}, badRequest("alamat hanya untuk pre-order yang diantar")
		}
		address, err = cleanOptional(request.Address, 300, "address")
		if err != nil {
			return res.PreorderResponse{}, err
		}
		if address == nil {
			return res.PreorderResponse{}, badRequest("alamat pengantaran wajib diisi")
		}
	}

	dueDate := p.DueDate
	if request.DueDate != nil {
		dueDate, err = checkScheduleDate(*request.DueDate, p.DueDate, "due_date", "tanggal jatuh tempo")
		if err != nil {
			return res.PreorderResponse{}, err
		}
	}
	dueTime := p.DueTime
	if request.DueTime != nil {
		dueTime, err = cleanTime(request.DueTime)
		if err != nil {
			return res.PreorderResponse{}, err
		}
	}
	notes := p.Notes
	if request.Notes != nil {
		notes, err = cleanOptional(request.Notes, 500, "notes")
		if err != nil {
			return res.PreorderResponse{}, err
		}
	}

	change := postgres.PreorderChange{
		CustomerName:  customerName,
		CustomerPhone: customerPhone,
		Address:       address,
		DueDate:       dueDate,
		DueTime:       dueTime,
		Notes:         notes,
		ShippingFee:   p.ShippingFee,
		OtherFee:      p.OtherFee,
		OtherFeeNote:  p.OtherFeeNote,
		// Perubahan digabungkan dengan p, jadi hanya boleh disimpan selama barisnya masih di versi p.
		Version: p.Version,
	}
	if request.ShippingFee != nil {
		change.ShippingFee = *request.ShippingFee
	}
	if request.OtherFee != nil {
		change.OtherFee = *request.OtherFee
	}
	if request.OtherFeeNote != nil {
		change.OtherFeeNote = request.OtherFeeNote
	}
	change.OtherFeeNote, err = cleanFees(p.Fulfillment, change.ShippingFee, change.OtherFee, change.OtherFeeNote)
	if err != nil {
		return res.PreorderResponse{}, err
	}

	if err := u.preorders.Update(id, change, postgres.Audit{ActorId: actorId}); err != nil {
		switch {
		case errors.Is(err, postgres.ErrPreorderClosed):
			return res.PreorderResponse{}, errPreorderClosed
		case errors.Is(err, postgres.ErrStale):
			return res.PreorderResponse{}, staleError("pre-order")
		case errors.Is(err, postgres.ErrPreorderOverpaid):
			return res.PreorderResponse{}, unprocessable(
				"total pesanan tidak boleh lebih kecil dari DP yang sudah diterima (Rp " +
					groupThousands(int(p.DownPayment)) + ")",
			)
		}
		var quota *postgres.QuotaExceededError
		if errors.As(err, &quota) {
			return res.PreorderResponse{}, quotaExceeded(quota)
		}
		return res.PreorderResponse{}, internalError(err)
	}
	return u.Get(id)
}

func (u *preorderUseCase) MarkReady(actorId, id string) (res.PreorderResponse, error) {
	p, err := u.find(id)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	switch p.Status {
	case model.PreorderReady:
		return res.PreorderResponse{}, &exception.GetError{Code: http.StatusConflict, Message: "pre-order sudah ditandai siap"}
	case model.PreorderOrdered:
		// lanjut
	default:
		return res.PreorderResponse{}, errPreorderClosed
	}

	if err := u.preorders.MarkReady(id, postgres.Audit{ActorId: actorId}); err != nil {
		if errors.Is(err, postgres.ErrPreorderClosed) {
			return res.PreorderResponse{}, errPreorderClosed
		}
		return res.PreorderResponse{}, internalError(err)
	}
	return u.Get(id)
}

// Complete menyerahkan barang: membuat transaksi biasa (stok siap produk PO berkurang, masuk omzet)
// untuk sisa pembayaran. Stok boleh menjadi minus, sama seperti penjualan di kasir.
// Harga memakai harga kesepakatan saat pre-order dibuat, bukan harga produk sekarang.
func (u *preorderUseCase) Complete(actorId, id string, request req.PreorderCompleteRequest) (res.TransactionResponse, error) {
	if err := u.validate.Struct(request); err != nil {
		return res.TransactionResponse{}, validationError(err)
	}
	if !validUUID(id) {
		return res.TransactionResponse{}, errPreorderNotFound
	}
	// Penyerahan yang sama = pre-order dan metode bayar yang sama. Uang yang diterima tidak ikut, karena kasir bisa
	// mengetik ulang nominalnya saat mencoba lagi.
	key, err := requestKey(request.RequestKey, actorId, actionPreorderComplete, struct {
		PreorderId    string `json:"preorder_id"`
		PaymentMethod string `json:"payment_method"`
	}{strings.ToLower(id), request.PaymentMethod})
	if err != nil {
		return res.TransactionResponse{}, err
	}
	if dup, err := earlierRequest(u.keys, key); err != nil || dup != nil {
		if err != nil {
			return res.TransactionResponse{}, err
		}
		return u.replayComplete(dup)
	}

	p, err := u.find(id)
	if err != nil {
		return res.TransactionResponse{}, err
	}
	if !isOpen(p) {
		return res.TransactionResponse{}, errPreorderClosed
	}

	// Nama penerima untuk pengiriman yang ikut terkirim; repository menolak tanpa nama selama pengirimannya masih
	// berjalan (ErrReceiverRequired).
	receivedBy, err := cleanOptional(&request.ReceivedBy, 100, "received_by")
	if err != nil {
		return res.TransactionResponse{}, err
	}

	// Nama dan harga diambil dari pesanan, yang terkunci sejak pesanan dibuat, bukan dari produk. Pesanan yang
	// produknya sudah dihapus pun tetap bisa diserahkan.
	items := make([]model.TransactionItem, 0, len(p.Items))
	for _, item := range p.Items {
		items = append(items, model.TransactionItem{
			ProductId:   item.ProductId,
			ProductName: item.ProductName,
			UnitPrice:   item.UnitPrice,
			Qty:         item.Qty,
			LineTotal:   item.LineTotal,
		})
	}
	// Urutan tetap supaya lock stok konsisten antar transaksi.
	sort.Slice(items, func(i, j int) bool { return items[i].ProductId < items[j].ProductId })

	remaining := p.Total - p.DownPayment
	method := request.PaymentMethod
	var paid, change int64
	switch {
	case remaining <= 0:
		// Sudah lunas lewat DP: tidak ada pembayaran baru.
		method = model.PaymentCash
		if n := len(p.Payments); n > 0 {
			method = p.Payments[n-1].Method
		}
	case method == "":
		return res.TransactionResponse{}, badRequest("metode bayar wajib diisi")
	case method == model.PaymentCash:
		if request.Paid < remaining {
			return res.TransactionResponse{}, unprocessable("pembayaran kurang dari sisa tagihan")
		}
		paid = request.Paid
		change = paid - remaining
	default:
		paid = remaining
	}

	created, err := u.preorders.Complete(id, p.DownPayment, receivedBy, model.Transaction{
		CashierId:     actorId,
		Subtotal:      p.Subtotal,
		ShippingFee:   p.ShippingFee,
		OtherFee:      p.OtherFee,
		OtherFeeNote:  p.OtherFeeNote,
		Total:         p.Total,
		DownPayment:   p.DownPayment,
		Paid:          paid,
		ChangeAmount:  change,
		PaymentMethod: method,
		Items:         items,
	}, key)
	if err != nil {
		var dup *postgres.DuplicateRequestError
		switch {
		case errors.Is(err, postgres.ErrPreorderClosed):
			return res.TransactionResponse{}, errPreorderClosed
		case errors.Is(err, postgres.ErrPreorderChanged):
			return res.TransactionResponse{}, &exception.GetError{
				Code:    http.StatusConflict,
				Message: "DP atau biaya pre-order berubah, muat ulang lalu coba lagi",
			}
		case errors.Is(err, sql.ErrNoRows):
			return res.TransactionResponse{}, errPreorderNotFound
		case errors.Is(err, postgres.ErrReceiverRequired):
			return res.TransactionResponse{}, badRequest("nama penerima wajib diisi (pengirimannya ikut ditandai terkirim)")
		case errors.As(err, &dup):
			return u.replayComplete(dup)
		}
		return res.TransactionResponse{}, internalError(err)
	}

	full, err := u.transactions.FindById(created.Id)
	if err != nil {
		return res.TransactionResponse{}, internalError(err)
	}
	return toTransactionResponse(full, true), nil
}

// replayComplete menjawab penyerahan yang dikirim ulang dengan kunci idempotensi yang sudah tersimpan: struk dari
// penyerahan itu, atau 409 kalau isinya berbeda.
func (u *preorderUseCase) replayComplete(dup *postgres.DuplicateRequestError) (res.TransactionResponse, error) {
	t, err := u.transactions.FindById(dup.TargetId)
	if err != nil {
		return res.TransactionResponse{}, internalError(err)
	}
	if !dup.Same {
		return res.TransactionResponse{}, duplicateRequest(
			"pre-order ini ternyata sudah diserahkan dengan struk " + t.ReceiptNo + ".",
		)
	}
	return toTransactionResponse(t, true), nil
}

// Cancel: admin boleh membatalkan pre-order mana pun, kasir hanya buatannya sendiri. Alasan wajib.
// DP dikembalikan ke pelanggan secara manual.
func (u *preorderUseCase) Cancel(actorId, role, id string, request req.PreorderCancelRequest) (res.PreorderResponse, error) {
	request.Reason = strings.TrimSpace(request.Reason)
	if err := u.validate.Struct(request); err != nil {
		return res.PreorderResponse{}, validationError(err)
	}
	p, err := u.find(id)
	if err != nil {
		return res.PreorderResponse{}, err
	}
	if role != model.RoleAdmin && p.CreatedBy != actorId {
		return res.PreorderResponse{}, &exception.GetError{
			Code:    http.StatusForbidden,
			Message: "kasir hanya dapat membatalkan pre-order buatannya sendiri",
		}
	}
	if !isOpen(p) {
		return res.PreorderResponse{}, errPreorderClosed
	}

	if err := u.preorders.Cancel(id, actorId, request.Reason); err != nil {
		if errors.Is(err, postgres.ErrPreorderClosed) {
			return res.PreorderResponse{}, errPreorderClosed
		}
		return res.PreorderResponse{}, internalError(err)
	}
	return u.Get(id)
}

func (u *preorderUseCase) find(id string) (model.Preorder, error) {
	if !validUUID(id) {
		return model.Preorder{}, errPreorderNotFound
	}
	p, err := u.preorders.FindById(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Preorder{}, errPreorderNotFound
		}
		return model.Preorder{}, internalError(err)
	}
	return p, nil
}

func isOpen(p model.Preorder) bool {
	return p.Status == model.PreorderOrdered || p.Status == model.PreorderReady
}

// cleanOptional merapikan teks opsional: spasi dibuang, kosong menjadi nil.
func cleanOptional(value *string, max int, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*value)
	if v == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(v) > max {
		return nil, badRequest(field + " terlalu panjang (maksimal " + strconv.Itoa(max) + " karakter)")
	}
	return &v, nil
}

// maxPreorderFee: batas ongkir dan biaya lain pre-order, sama dengan batas ongkir pengiriman.
const maxPreorderFee = maxDeliveryFee

// cleanFees memeriksa ongkir dan biaya lain, lalu mengembalikan keterangan biaya lain yang sudah dirapikan.
// Ongkir hanya untuk pesanan diantar. Keterangan dibuang kalau biaya lainnya 0.
func cleanFees(fulfillment string, shippingFee, otherFee int64, otherFeeNote *string) (*string, error) {
	if shippingFee < 0 || shippingFee > maxPreorderFee {
		return nil, badRequest("ongkir tidak valid (0 sampai 100.000.000)")
	}
	if otherFee < 0 || otherFee > maxPreorderFee {
		return nil, badRequest("biaya lain tidak valid (0 sampai 100.000.000)")
	}
	if shippingFee > 0 && fulfillment != model.FulfillmentDelivery {
		return nil, badRequest("ongkir hanya untuk pesanan yang diantar")
	}
	if otherFee == 0 {
		return nil, nil
	}
	return cleanOptional(otherFeeNote, 40, "keterangan biaya lain")
}

// cleanTime memastikan format HH:MM. Kosong berarti tidak ada jam.
func cleanTime(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*value)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse("15:04", v)
	if err != nil {
		return nil, badRequest("jam tidak valid, gunakan format HH:MM")
	}
	out := t.Format("15:04")
	return &out, nil
}

func toPreorderResponse(p model.Preorder, withDetail bool) res.PreorderResponse {
	remaining := p.Total - p.DownPayment
	if remaining < 0 {
		remaining = 0
	}
	response := res.PreorderResponse{
		Id:             p.Id,
		OrderNo:        p.OrderNo,
		CustomerName:   p.CustomerName,
		CustomerPhone:  p.CustomerPhone,
		DueDate:        p.DueDate,
		DueTime:        p.DueTime,
		Fulfillment:    p.Fulfillment,
		Address:        p.Address,
		Notes:          p.Notes,
		Status:         p.Status,
		Subtotal:       p.Subtotal,
		ShippingFee:    p.ShippingFee,
		OtherFee:       p.OtherFee,
		OtherFeeNote:   p.OtherFeeNote,
		Total:          p.Total,
		DownPayment:    p.DownPayment,
		Remaining:      remaining,
		CreatedById:    p.CreatedBy,
		CreatedByName:  p.CreatedByName,
		CreatedAt:      p.CreatedAt,
		TransactionId:  p.TransactionId,
		CancelledAt:    p.CancelledAt,
		CancelReason:   p.CancelReason,
		Version:        p.Version,
		DeliveryNo:     p.DeliveryNo,
		DeliveryStatus: p.DeliveryStatus,
		ReceivedBy:     p.ReceivedBy,
	}
	if withDetail {
		response.Items = make([]res.PreorderItemResponse, 0, len(p.Items))
		for _, item := range p.Items {
			response.Items = append(response.Items, res.PreorderItemResponse{
				Id:          item.Id,
				ProductId:   item.ProductId,
				ProductName: item.ProductName,
				UnitPrice:   item.UnitPrice,
				Qty:         item.Qty,
				LineTotal:   item.LineTotal,
			})
		}
		response.Payments = make([]res.PreorderPaymentResponse, 0, len(p.Payments))
		for _, pay := range p.Payments {
			response.Payments = append(response.Payments, res.PreorderPaymentResponse{
				Id:            pay.Id,
				Amount:        pay.Amount,
				Method:        pay.Method,
				CreatedByName: pay.CreatedByName,
				CreatedAt:     pay.CreatedAt,
			})
		}
	}
	return response
}

// NewPreorderUseCase Constructor
func NewPreorderUseCase(
	preorders postgres.PreorderRepository,
	transactions postgres.TransactionRepository,
	products postgres.ProductRepository,
	keys postgres.IdempotencyRepository,
) PreorderUseCase {
	return &preorderUseCase{
		preorders:    preorders,
		transactions: transactions,
		products:     products,
		keys:         keys,
		validate:     newValidator(),
	}
}
