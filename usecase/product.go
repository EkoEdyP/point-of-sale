package usecase

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/EkoEdyP/point-of-sale/utility/common"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

var (
	errProductNotFound         = &exception.GetError{Code: http.StatusNotFound, Message: "produk tidak ditemukan"}
	errPreorderProductNotFound = &exception.GetError{Code: http.StatusNotFound, Message: "produk PO tidak ditemukan"}
)

type (
	ProductUseCase interface {
		Create(actorId string, request req.ProductRequest) (res.ProductResponse, error)
		Update(actorId, id string, request req.ProductRequest) (res.ProductResponse, error)
		Delete(actorId, id string) error
		List(role string, query req.ProductListQuery) (res.ProductListResponse, error)
		AddProduction(actorId, id string, request req.ProductionRequest) (res.ProductResponse, error)
	}
	productUseCase struct {
		products postgres.ProductRepository
		keys     postgres.IdempotencyRepository
		validate *validator.Validate
	}
)

func (u *productUseCase) Create(actorId string, request req.ProductRequest) (res.ProductResponse, error) {
	normalizeProductRequest(&request)
	if err := u.validate.Struct(request); err != nil {
		return res.ProductResponse{}, validationError(err)
	}
	if err := checkWholesale(request); err != nil {
		return res.ProductResponse{}, err
	}
	if request.Stock < 0 {
		return res.ProductResponse{}, badRequest("stok awal tidak boleh minus")
	}

	active := true
	if request.Active != nil {
		active = *request.Active
	}
	product := toProduct(request)
	product.Id = common.GenerateID()
	product.Kind = model.ProductStore
	if request.Kind != "" {
		product.Kind = request.Kind
	}
	product.Active = active

	saved, err := u.products.Save(product, postgres.Audit{ActorId: actorId})
	if err != nil {
		return res.ProductResponse{}, mapProductWriteError(err)
	}
	return toProductResponse(saved), nil
}

func (u *productUseCase) Update(actorId, id string, request req.ProductRequest) (res.ProductResponse, error) {
	if !validUUID(id) {
		return res.ProductResponse{}, errProductNotFound
	}
	normalizeProductRequest(&request)
	if err := u.validate.Struct(request); err != nil {
		return res.ProductResponse{}, validationError(err)
	}
	if err := checkWholesale(request); err != nil {
		return res.ProductResponse{}, err
	}

	stockChange := 0
	if request.StockBefore != nil {
		stockChange = request.Stock - *request.StockBefore
	}
	saved, err := u.products.Update(id, toProduct(request), stockChange, request.Active, postgres.Audit{ActorId: actorId})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res.ProductResponse{}, errProductNotFound
		}
		return res.ProductResponse{}, mapProductWriteError(err)
	}
	return toProductResponse(saved), nil
}

func (u *productUseCase) Delete(actorId, id string) error {
	if !validUUID(id) {
		return errProductNotFound
	}
	if err := u.products.SoftDelete(id, postgres.Audit{ActorId: actorId}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errProductNotFound
		}
		return internalError(err)
	}
	return nil
}

// List: admin melihat semua produk (aktif dan nonaktif), kecuali meminta OnlyActive. Kasir hanya melihat produk
// aktif. Satu daftar hanya berisi satu jenis produk.
func (u *productUseCase) List(role string, query req.ProductListQuery) (res.ProductListResponse, error) {
	kind := strings.TrimSpace(query.Kind)
	switch kind {
	case "":
		kind = model.ProductStore
	case model.ProductStore, model.ProductPreorder:
		// valid
	default:
		return res.ProductListResponse{}, badRequest("kind tidak valid (store atau preorder)")
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
	isAdmin := role == model.RoleAdmin
	if len(query.Ids) > 100 {
		return res.ProductListResponse{}, badRequest("ids maksimal 100")
	}
	for _, id := range query.Ids {
		if !validUUID(id) {
			return res.ProductListResponse{}, badRequest("ids tidak valid")
		}
	}

	products, total, err := u.products.List(postgres.ProductFilter{
		Kind:          kind,
		Search:        strings.TrimSpace(query.Search),
		Ids:           query.Ids,
		OnlyActive:    !isAdmin || query.OnlyActive,
		NegativeStock: query.NegativeStock,
		Limit:         limit,
		Offset:        (page - 1) * limit,
	})
	if err != nil {
		return res.ProductListResponse{}, internalError(err)
	}

	data := make([]res.ProductResponse, 0, len(products))
	for _, p := range products {
		data = append(data, toProductResponse(p))
	}
	return res.ProductListResponse{Data: data, Page: page, Limit: limit, Total: total}, nil
}

// AddProduction mencatat hasil produksi produk PO: stok siap bertambah, lalu dipakai pre-order
// dengan hari H paling awal lebih dulu (lihat PreorderUseCase.Production).
func (u *productUseCase) AddProduction(actorId, id string, request req.ProductionRequest) (res.ProductResponse, error) {
	if !validUUID(id) {
		return res.ProductResponse{}, errPreorderProductNotFound
	}
	if err := u.validate.Struct(request); err != nil {
		return res.ProductResponse{}, validationError(err)
	}
	key, err := requestKey(request.RequestKey, actorId, actionProduction, struct {
		ProductId string `json:"product_id"`
		Qty       int    `json:"qty"`
	}{strings.ToLower(id), request.Qty})
	if err != nil {
		return res.ProductResponse{}, err
	}
	if dup, err := earlierRequest(u.keys, key); err != nil || dup != nil {
		if err != nil {
			return res.ProductResponse{}, err
		}
		return u.replayProduction(dup)
	}

	saved, err := u.products.AddPreorderStock(id, request.Qty, postgres.Audit{ActorId: actorId}, key)
	if err != nil {
		var dup *postgres.DuplicateRequestError
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return res.ProductResponse{}, errPreorderProductNotFound
		case errors.As(err, &dup):
			return u.replayProduction(dup)
		}
		return res.ProductResponse{}, internalError(err)
	}
	return toProductResponse(saved), nil
}

// replayProduction menjawab hasil produksi yang dikirim ulang dengan kunci idempotensi yang sudah tersimpan:
// produknya apa adanya (stoknya sudah bertambah sekali), atau 409 kalau isinya berbeda.
func (u *productUseCase) replayProduction(dup *postgres.DuplicateRequestError) (res.ProductResponse, error) {
	if !dup.Same {
		return res.ProductResponse{}, duplicateRequest(
			"hasil produksi sebelumnya ternyata sudah tercatat. Periksa stok siap sebelum menambah lagi.",
		)
	}
	found, err := u.products.FindByIds([]string{dup.TargetId})
	if err != nil {
		return res.ProductResponse{}, internalError(err)
	}
	if len(found) == 0 {
		return res.ProductResponse{}, errPreorderProductNotFound
	}
	return toProductResponse(found[0]), nil
}

// normalizeProductRequest merapikan input: spasi dibuang, SKU kosong dianggap tidak diisi (NULL), supaya dua produk
// tanpa SKU tidak bentrok di unique index. Batas harga grosir kosong menjadi 50.
func normalizeProductRequest(r *req.ProductRequest) {
	r.Name = strings.TrimSpace(r.Name)
	r.Sku = trimToNil(r.Sku)
	r.CategoryId = trimToNil(r.CategoryId)
	if r.WholesaleMinQty == nil {
		minQty := model.DefaultWholesaleMinQty
		r.WholesaleMinQty = &minQty
	}
}

// checkWholesale memeriksa harga grosir sesudah validasi struct (Price sudah pasti diisi): harga grosir harus
// lebih dari 0 dan lebih murah dari harga jual, batasnya 2 sampai 10000 pcs (jumlah maksimal per produk).
func checkWholesale(r req.ProductRequest) error {
	if minQty := *r.WholesaleMinQty; minQty < 2 || minQty > maxQtyPerProduct {
		return badRequest(fmt.Sprintf("harga grosir berlaku mulai 2 sampai %d pcs", maxQtyPerProduct))
	}
	if r.WholesalePrice == nil {
		return nil
	}
	if *r.WholesalePrice <= 0 {
		return badRequest("harga grosir harus lebih dari 0")
	}
	if *r.WholesalePrice >= *r.Price {
		return badRequest("harga grosir harus lebih murah dari harga jual")
	}
	return nil
}

func trimToNil(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

// toProduct mengisi field dari request. Price sudah divalidasi tidak nil, WholesaleMinQty sudah diisi normalize.
func toProduct(r req.ProductRequest) model.Product {
	return model.Product{
		Sku:             r.Sku,
		Name:            r.Name,
		CategoryId:      r.CategoryId,
		Price:           *r.Price,
		WholesalePrice:  r.WholesalePrice,
		WholesaleMinQty: *r.WholesaleMinQty,
		Stock:           r.Stock,
	}
}

func toProductResponse(p model.Product) res.ProductResponse {
	return res.ProductResponse{
		Id:              p.Id,
		Kind:            p.Kind,
		Sku:             p.Sku,
		Name:            p.Name,
		CategoryId:      p.CategoryId,
		Price:           p.Price,
		WholesalePrice:  p.WholesalePrice,
		WholesaleMinQty: p.WholesaleMinQty,
		Stock:           p.Stock,
		Active:          p.Active,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

func mapProductWriteError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch {
		case pqErr.Code == "23505" && pqErr.Constraint == "products_sku_active_idx":
			return &exception.GetError{Code: http.StatusConflict, Message: "sku sudah dipakai"}
		case pqErr.Code == "23503":
			return &exception.GetError{Code: http.StatusBadRequest, Message: "kategori tidak ditemukan"}
		}
	}
	return internalError(err)
}

// validUUID memastikan id dari URL berformat UUID baku, supaya Postgres tidak error "invalid input syntax".
func validUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	_, err := uuid.Parse(id)
	return err == nil
}

// NewProductUseCase Constructor
func NewProductUseCase(products postgres.ProductRepository, keys postgres.IdempotencyRepository) ProductUseCase {
	return &productUseCase{
		products: products,
		keys:     keys,
		validate: newValidator(),
	}
}
