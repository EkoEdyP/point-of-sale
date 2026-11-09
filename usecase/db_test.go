package usecase_test

// Uji dengan database percobaan pos_check, lewat usecase dan repository asli. Hanya jalan kalau APP_DB_NAME=pos_check,
// jadi database toko tidak pernah dipakai; tanpa itu semuanya dilewati (skip). Cara menjalankan: `make test-db`
// (membuat ulang pos_check dari migrasi). Data uji memakai nama unik per putaran, jadi bisa diulang tanpa reset.

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EkoEdyP/point-of-sale/config"
	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/manager"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/EkoEdyP/point-of-sale/usecase"
)

type testEnv struct {
	db      *sql.DB
	repos   manager.RepositoryManager
	uc      manager.UseCaseManager
	adminID string
	run     string // akhiran unik putaran ini
	n       atomic.Int64
}

var (
	envOnce sync.Once
	env     *testEnv
	envErr  error
)

func setup(t *testing.T) *testEnv {
	t.Helper()
	if os.Getenv("APP_DB_NAME") != "pos_check" {
		t.Skip("butuh database percobaan: jalankan `make test-db` (APP_DB_NAME=pos_check)")
	}
	envOnce.Do(func() {
		cfg, err := config.NewConfig()
		if err != nil {
			envErr = err
			return
		}
		infra, err := manager.NewInfraManager(cfg)
		if err != nil {
			envErr = err
			return
		}
		repos := manager.NewRepositoryManager(infra)
		e := &testEnv{db: infra.Conn(), repos: repos, uc: manager.NewUseCaseManager(repos, cfg.JWTConfig)}
		e.run = fmt.Sprintf("%x", time.Now().UnixNano()%(1<<32))
		// pos_check dibuat kosong: admin untuk membuat data uji dibuat di sini kalau belum ada.
		err = e.db.QueryRow(`SELECT id FROM users WHERE username = 'admin-uji'`).Scan(&e.adminID)
		if errors.Is(err, sql.ErrNoRows) {
			err = e.db.QueryRow(`INSERT INTO users(name, username, password_hash, role)
				VALUES ('Admin Uji', 'admin-uji', 'x', 'admin') RETURNING id`).Scan(&e.adminID)
		}
		env, envErr = e, err
	})
	if envErr != nil {
		t.Fatal(envErr)
	}
	return env
}

func (e *testEnv) name(prefix string) string {
	return fmt.Sprintf("%s %s-%d", prefix, e.run, e.n.Add(1))
}

func (e *testEnv) cashier(t *testing.T) res.UserResponse {
	t.Helper()
	n := e.n.Add(1)
	u, err := e.uc.AuthUseCase().Register(e.adminID, req.RegisterRequest{
		Name: fmt.Sprintf("Kasir %s-%d", e.run, n), Username: fmt.Sprintf("kasir%s%d", e.run, n),
		Password: "kasir-uji-rahasia-1", Role: model.RoleCashier,
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (e *testEnv) product(t *testing.T, r req.ProductRequest) string {
	t.Helper()
	r.Name = e.name(r.Name)
	p, err := e.uc.ProductUseCase().Create(e.adminID, r)
	if err != nil {
		t.Fatal(err)
	}
	return p.Id
}

func (e *testEnv) stock(t *testing.T, productID string) int {
	t.Helper()
	return one[int](t, e.db, `SELECT stock FROM products WHERE id = $1`, productID)
}

// preorder: 2 pcs produk PO, DP Rp 5.000, hari H lusa (diantar = dengan alamat dan ongkir Rp 5.000).
func (e *testEnv) preorder(t *testing.T, actorID, productID, fulfillment string) res.PreorderResponse {
	t.Helper()
	r := req.PreorderRequest{CustomerName: e.name("Bu"), DueDate: time.Now().AddDate(0, 0, 2).Format("2006-01-02"),
		Fulfillment: fulfillment, Items: []req.PreorderItemRequest{{ProductId: productID, Qty: 2}},
		DownPayment: 5000, DownPaymentMethod: "cash"}
	if fulfillment == "delivery" {
		r.Address, r.ShippingFee = ptr("Jl. Uji 1"), 5000
	}
	p, err := e.uc.PreorderUseCase().Create(actorID, r)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (e *testEnv) deliveryOf(t *testing.T, preorderID string) string {
	t.Helper()
	return one[string](t, e.db, `SELECT id FROM deliveries WHERE preorder_id = $1`, preorderID)
}

func one[T any](t *testing.T, db *sql.DB, query string, args ...any) T {
	t.Helper()
	var v T
	if err := db.QueryRow(query, args...).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func ptr[T any](v T) *T { return &v }

func newKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// errOf: status dan kode (Reason) dari error usecase.
func errOf(err error) (int, string) {
	var ge *exception.GetError
	if errors.As(err, &ge) {
		return ge.Code, ge.Reason
	}
	return 0, ""
}

func sellOne(productID string, qty int) req.TransactionRequest {
	return req.TransactionRequest{Items: []req.TransactionItemRequest{{ProductId: productID, Qty: qty}},
		PaymentMethod: "cash", Paid: 1_000_000}
}

func TestSaleAndVoidKeepStock(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	teh := e.product(t, req.ProductRequest{Name: "Teh", Price: ptr(int64(5000)), Stock: 10})
	sales := e.uc.TransactionUseCase()

	sale, err := sales.Create(kasir.Id, req.TransactionRequest{
		Items: []req.TransactionItemRequest{{ProductId: teh, Qty: 3}}, PaymentMethod: "cash", Paid: 20000})
	if err != nil {
		t.Fatal(err)
	}
	if sale.Total != 15000 || sale.Change != 5000 || e.stock(t, teh) != 7 {
		t.Fatalf("total %d, kembalian %d, stok %d; mau 15000, 5000, 7", sale.Total, sale.Change, e.stock(t, teh))
	}
	voided, err := sales.Void(kasir.Id, model.RoleCashier, sale.Id, req.VoidRequest{Reason: "salah input"})
	if err != nil {
		t.Fatal(err)
	}
	by := voided.VoidedByName
	if voided.Status != "void" || e.stock(t, teh) != 10 || by == nil || *by != kasir.Name {
		t.Fatalf("setelah batal: status %s, stok %d, oleh %v", voided.Status, e.stock(t, teh), by)
	}
	if _, err := sales.Void(kasir.Id, model.RoleCashier, sale.Id, req.VoidRequest{Reason: "lagi"}); err == nil {
		t.Fatal("membatalkan dua kali harus ditolak")
	}
	if e.stock(t, teh) != 10 {
		t.Fatalf("stok berubah karena pembatalan kedua: %d", e.stock(t, teh))
	}
}

func TestSaleRetryWithSameKey(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	teh := e.product(t, req.ProductRequest{Name: "Teh", Price: ptr(int64(5000)), Stock: 10})
	sales := e.uc.TransactionUseCase()

	r := sellOne(teh, 2)
	r.RequestKey = newKey()
	first, err := sales.Create(kasir.Id, r)
	if err != nil {
		t.Fatal(err)
	}
	again, err := sales.Create(kasir.Id, r)
	if err != nil || again.Id != first.Id || e.stock(t, teh) != 8 {
		t.Fatalf("kirim ulang: %v, struk %s vs %s, stok %d; mau struk sama dan stok 8", err, again.Id, first.Id,
			e.stock(t, teh))
	}
	changed := r
	changed.Items = []req.TransactionItemRequest{{ProductId: teh, Qty: 3}}
	_, err = sales.Create(kasir.Id, changed)
	if code, reason := errOf(err); code != http.StatusConflict || reason != "duplicate_request" {
		t.Fatalf("kunci sama, isi beda: %v; mau 409 duplicate_request", err)
	}
}

func TestSalePriceChanged(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	teh := e.product(t, req.ProductRequest{Name: "Teh", Price: ptr(int64(5000)), Stock: 10})
	r := sellOne(teh, 1)
	r.ExpectedTotal = ptr(int64(4000))
	_, err := e.uc.TransactionUseCase().Create(kasir.Id, r)
	if code, reason := errOf(err); code != http.StatusConflict || reason != "price_changed" || e.stock(t, teh) != 10 {
		t.Fatalf("total di layar beda: %v (stok %d); mau 409 price_changed tanpa mengurangi stok", err, e.stock(t, teh))
	}
}

func TestWholesalePriceFromMinQty(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	gula := e.product(t, req.ProductRequest{Name: "Gula", Price: ptr(int64(5000)), Stock: 100,
		WholesalePrice: ptr(int64(4000)), WholesaleMinQty: ptr(10)})
	sales := e.uc.TransactionUseCase()
	for qty, want := range map[int]int64{9: 45000, 10: 40000} {
		sale, err := sales.Create(kasir.Id, sellOne(gula, qty))
		if err != nil || sale.Total != want {
			t.Fatalf("%d pcs: total %d, %v; mau %d", qty, sale.Total, err, want)
		}
	}
}

func TestPreorderHandover(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	po := e.preorder(t, kasir.Id, bolu, "pickup")
	if po.Total != 20000 || po.DownPayment != 5000 || po.Remaining != 15000 {
		t.Fatalf("PO: total %d, DP %d, sisa %d", po.Total, po.DownPayment, po.Remaining)
	}
	sale, err := e.uc.PreorderUseCase().Complete(kasir.Id, po.Id, req.PreorderCompleteRequest{PaymentMethod: "cash",
		Paid: 15000})
	if err != nil {
		t.Fatal(err)
	}
	if sale.Source != model.SourcePreorder || sale.DownPayment != 5000 || sale.Total != 20000 || e.stock(t, bolu) != -2 {
		t.Fatalf("struk penyerahan: sumber %s, DP %d, total %d, stok PO %d", sale.Source, sale.DownPayment, sale.Total,
			e.stock(t, bolu))
	}
	done, _ := e.uc.PreorderUseCase().Get(po.Id)
	if done.Status != "completed" || done.TransactionId == nil || *done.TransactionId != sale.Id {
		t.Fatalf("PO setelah diserahkan: %s, struk %v", done.Status, done.TransactionId)
	}
	_, err = e.uc.TransactionUseCase().Void(e.adminID, model.RoleAdmin, sale.Id, req.VoidRequest{Reason: "uji"})
	if code, _ := errOf(err); code != http.StatusConflict {
		t.Fatalf("struk penyerahan PO dibatalkan: %v; mau 409", err)
	}
}

// Setiap DP, termasuk DP awal saat PO dibuat, masuk riwayat perubahan PO-nya: pelaku, jumlah, metode, dan sisa
// tagihan sebelum dan sesudahnya. DP yang dikirim ulang dengan kunci yang sama tidak tercatat dua kali.
func TestPaymentsInHistory(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	po := e.preorder(t, kasir.Id, bolu, "pickup") // total Rp 20.000, DP awal Rp 5.000 tunai

	pay := req.PreorderPaymentRequest{Amount: 7000, Method: "qris", RequestKey: newKey()}
	for range 2 {
		if _, err := e.uc.PreorderUseCase().AddPayment(e.adminID, po.Id, pay); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := e.db.Query(`SELECT actor_id, entity_label, changes::text FROM audit_logs
		WHERE entity = 'preorder' AND action = 'payment' AND entity_id = $1 ORDER BY id`, po.Id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var actor, label, changes string
		if err := rows.Scan(&actor, &label, &changes); err != nil {
			t.Fatal(err)
		}
		got = append(got, actor+" "+label+" "+changes)
	}
	want := []string{
		kasir.Id + " " + po.OrderNo + ` {"amount": [null, 5000], "method": [null, "cash"], "remaining": [20000, 15000]}`,
		e.adminID + " " + po.OrderNo + ` {"amount": [null, 7000], "method": [null, "qris"], "remaining": [15000, 8000]}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("riwayat DP:\n%s\nmau:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDeliveryFollowsPreorder(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	deliveries := e.uc.DeliveryUseCase()

	po := e.preorder(t, kasir.Id, bolu, "delivery")
	// Tanpa nama penerima ditolak sebelum struknya dibuat, jadi nomor struk tidak terlewat.
	receipts := one[int64](t, e.db, `SELECT last_value FROM receipt_seq`)
	done := req.PreorderCompleteRequest{PaymentMethod: "cash", Paid: 20000}
	_, err := e.uc.PreorderUseCase().Complete(kasir.Id, po.Id, done)
	if code, _ := errOf(err); code != http.StatusBadRequest {
		t.Fatalf("PO diantar diselesaikan tanpa nama penerima: %v; mau 400", err)
	}
	if after := one[int64](t, e.db, `SELECT last_value FROM receipt_seq`); after != receipts {
		t.Fatalf("nomor struk %d -> %d setelah ditolak; mau tetap", receipts, after)
	}
	done.ReceivedBy = "  Pak Satpam "
	if _, err := e.uc.PreorderUseCase().Complete(kasir.Id, po.Id, done); err != nil {
		t.Fatal(err)
	}
	d, _ := deliveries.Get(e.deliveryOf(t, po.Id))
	if d.Status != model.DeliveryDelivered || d.ReceivedBy == nil || *d.ReceivedBy != "Pak Satpam" {
		t.Fatalf("PO diantar selesai, pengirimannya %s diterima %v; mau delivered, Pak Satpam", d.Status, d.ReceivedBy)
	}
	if got := one[string](t, e.db, `SELECT changes::text FROM audit_logs WHERE entity = 'delivery' AND entity_id = $1
		AND action = 'deliver'`, d.Id); !strings.Contains(got, `"received_by": [null, "Pak Satpam"]`) {
		t.Fatalf("riwayat terkirim = %s; mau berisi nama penerima", got)
	}
	po2 := e.preorder(t, kasir.Id, bolu, "delivery")
	if _, err := e.uc.PreorderUseCase().Cancel(kasir.Id, model.RoleCashier, po2.Id,
		req.PreorderCancelRequest{Reason: "pelanggan batal"}); err != nil {
		t.Fatal(err)
	}
	if d, _ := deliveries.Get(e.deliveryOf(t, po2.Id)); d.Status != model.DeliveryCancelled {
		t.Fatalf("PO dibatalkan, pengirimannya %s; mau cancelled", d.Status)
	}
}

func TestPreorderEditFromOldVersion(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	preorders := e.uc.PreorderUseCase()
	po := e.preorder(t, kasir.Id, bolu, "pickup")

	saved, err := preorders.Update(e.adminID, po.Id, req.PreorderUpdateRequest{Notes: ptr("A"), Version: ptr(po.Version)})
	if err != nil || saved.Version != po.Version+1 {
		t.Fatalf("simpan dari versi terbaru: %v, versi %d", err, saved.Version)
	}
	_, err = preorders.Update(e.adminID, po.Id, req.PreorderUpdateRequest{Notes: ptr("lama"), Version: ptr(po.Version)})
	if code, reason := errOf(err); code != http.StatusConflict || reason != "stale" {
		t.Fatalf("simpan dari versi lama: %v; mau 409 stale", err)
	}
	if notes := one[string](t, e.db, `SELECT notes FROM preorders WHERE id = $1`, po.Id); notes != "A" {
		t.Fatalf("catatan tertimpa data lama: %q", notes)
	}

	// Delapan orang menyimpan dari versi yang sama pada saat bersamaan: hanya satu yang tersimpan.
	var ok, stale atomic.Int32
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := preorders.Update(e.adminID, po.Id, req.PreorderUpdateRequest{Notes: ptr(fmt.Sprint(i)),
				Version: ptr(saved.Version)})
			if err == nil {
				ok.Add(1)
			} else if _, reason := errOf(err); reason == "stale" {
				stale.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || stale.Load() != 7 {
		t.Fatalf("8 simpan bersamaan: %d tersimpan, %d stale; mau 1 dan 7", ok.Load(), stale.Load())
	}
}

// Pengiriman pre-order berbagi pemesan, alamat, jadwal, dan catatan dengan pre-order-nya: diubah dari mana pun,
// keduanya tetap sama, dan riwayat yang satunya mencatat dari mana perubahan itu.
func TestDeliverySharesPreorderData(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	preorders, deliveries := e.uc.PreorderUseCase(), e.uc.DeliveryUseCase()
	day := func(offset int) string { return time.Now().AddDate(0, 0, offset).Format("2006-01-02") }
	po := e.preorder(t, kasir.Id, bolu, "delivery")
	id := e.deliveryOf(t, po.Id)
	shared := func(want string) {
		t.Helper()
		got := one[string](t, e.db, `SELECT concat_ws('|', p.customer_name, p.customer_phone, p.address, p.due_date,
			p.due_time, p.notes) || ' = ' || concat_ws('|', d.customer_name, d.customer_phone, d.address,
			d.scheduled_date, d.scheduled_time, d.notes)
			FROM preorders p JOIN deliveries d ON d.preorder_id = p.id WHERE p.id = $1`, po.Id)
		if got != want+" = "+want {
			t.Fatalf("pre-order = pengiriman: %s; mau %s di keduanya", got, want)
		}
	}
	lastEntry := func(entity, entityID string) string {
		t.Helper()
		return one[string](t, e.db, `SELECT coalesce(note, '') || ' ' || changes::text FROM audit_logs
			WHERE entity = $1 AND entity_id = $2 ORDER BY id DESC LIMIT 1`, entity, entityID)
	}

	// Dari pengiriman: semua data bersama ikut ke pre-order, ongkirnya tetap di pre-order.
	d, err := deliveries.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deliveries.Update(e.adminID, id, req.DeliveryUpdateRequest{CustomerName: ptr("Pak Amir"),
		CustomerPhone: ptr("0812"), Address: ptr("Jl. Baru 2"), ScheduledDate: ptr(day(3)), ScheduledTime: ptr("10:00"),
		Notes: ptr("pagar hijau"), Fee: ptr(d.Fee), Version: ptr(d.Version)}); err != nil {
		t.Fatal(err)
	}
	shared("Pak Amir|0812|Jl. Baru 2|" + day(3) + "|10:00:00|pagar hijau")
	if got := lastEntry("preorder", po.Id); !strings.HasPrefix(got, "Ikut pengiriman "+d.DeliveryNo+" ") ||
		!strings.Contains(got, `"customer_name": [`) || !strings.Contains(got, `"due_date": [`) {
		t.Fatalf("riwayat pre-order = %s; mau perubahannya dengan nomor pengiriman", got)
	}
	if fee := one[int64](t, e.db, `SELECT fee FROM deliveries WHERE id = $1`, id); fee != 0 {
		t.Fatalf("kolom ongkir pengiriman PO = %d; mau tetap 0 (ongkirnya di pre-order)", fee)
	}
	// Formulir pre-order yang dibuka sebelum perubahan itu sudah basi.
	_, err = preorders.Update(e.adminID, po.Id, req.PreorderUpdateRequest{Notes: ptr("lama"), Version: ptr(po.Version)})
	if _, reason := errOf(err); reason != "stale" {
		t.Fatalf("ubah pre-order dari versi lama: %v; mau stale", err)
	}

	// Dari pre-order: data pelanggan dan catatan ikut ke pengiriman.
	p, err := preorders.Get(po.Id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := preorders.Update(e.adminID, po.Id, req.PreorderUpdateRequest{CustomerName: ptr("Bu Sari"),
		Address: ptr("Jl. Lama 3"), Notes: ptr("titip satpam"), Version: ptr(p.Version)}); err != nil {
		t.Fatal(err)
	}
	shared("Bu Sari|0812|Jl. Lama 3|" + day(3) + "|10:00:00|titip satpam")
	if got := lastEntry("delivery", id); !strings.HasPrefix(got, "Ikut pre-order "+po.OrderNo+" ") {
		t.Fatalf("riwayat pengiriman = %s; mau perubahannya dengan nomor pre-order", got)
	}
	// Ongkir tidak ikut data bersama: pengirimannya tidak berubah, jadi versinya tetap.
	p, _ = preorders.Get(po.Id)
	before, _ := deliveries.Get(id)
	if _, err := preorders.Update(e.adminID, po.Id, req.PreorderUpdateRequest{ShippingFee: ptr(int64(7000)),
		Version: ptr(p.Version)}); err != nil {
		t.Fatal(err)
	}
	if after, _ := deliveries.Get(id); after.Version != before.Version || after.Fee != 7000 {
		t.Fatalf("ubah ongkir: versi pengiriman %d -> %d, ongkir %d; mau versi tetap, ongkir 7000",
			before.Version, after.Version, after.Fee)
	}

	// Gagal lalu dijadwalkan ulang dari pengiriman: hari H pre-order ikut pindah.
	if _, err := deliveries.Fail(e.adminID, id, req.DeliveryReasonRequest{Reason: "rumah kosong"}); err != nil {
		t.Fatal(err)
	}
	if _, err := deliveries.Update(e.adminID, id, req.DeliveryUpdateRequest{ScheduledDate: ptr(day(4))}); err != nil {
		t.Fatal(err)
	}
	shared("Bu Sari|0812|Jl. Lama 3|" + day(4) + "|10:00:00|titip satpam")

	// Pindah ke tanggal yang kuotanya tidak cukup: ditolak seperti mengubah hari H, dan tidak ada yang berubah.
	full := time.Now().AddDate(0, 0, 400+int(time.Now().UnixNano()%300)).Format("2006-01-02")
	q, err := preorders.Quota(full, "")
	if err != nil {
		t.Fatal(err)
	}
	restore := func() {
		_, _ = preorders.SetDailyLimit(e.adminID, req.PreorderDailyLimitRequest{DailyLimit: q.DailyLimit})
	}
	t.Cleanup(restore)
	if _, err := preorders.SetDailyLimit(e.adminID, req.PreorderDailyLimitRequest{DailyLimit: q.Used + 1}); err != nil {
		t.Fatal(err)
	}
	_, err = deliveries.Update(e.adminID, id, req.DeliveryUpdateRequest{ScheduledDate: ptr(full)})
	restore()
	if code, _ := errOf(err); code != http.StatusUnprocessableEntity {
		t.Fatalf("pindah ke tanggal yang kuotanya tidak cukup: %v; mau 422", err)
	}
	shared("Bu Sari|0812|Jl. Lama 3|" + day(4) + "|10:00:00|titip satpam")
}

// Aturan tanggal yang sama untuk pre-order dan pengiriman: tanggal yang sudah lewat boleh tetap (tidak berubah),
// tapi tanggal baru dan penjadwalan ulang pengiriman gagal tidak boleh sebelum hari ini.
func TestScheduleDateRules(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	preorders, deliveries := e.uc.PreorderUseCase(), e.uc.DeliveryUseCase()
	day := func(offset int) string { return time.Now().AddDate(0, 0, offset).Format("2006-01-02") }

	po := e.preorder(t, kasir.Id, bolu, "pickup")
	if _, err := e.db.Exec(`UPDATE preorders SET due_date = $2 WHERE id = $1`, po.Id, day(-3)); err != nil {
		t.Fatal(err)
	}
	if _, err := preorders.Update(e.adminID, po.Id, req.PreorderUpdateRequest{DueDate: ptr(day(-3)),
		Notes: ptr("tetap tanggal lama")}); err != nil {
		t.Fatalf("PO terlambat, tanggal tidak diubah: %v; mau tersimpan", err)
	}
	_, err := preorders.Update(e.adminID, po.Id, req.PreorderUpdateRequest{DueDate: ptr(day(-2))})
	if code, _ := errOf(err); code != http.StatusBadRequest {
		t.Fatalf("PO dipindah ke tanggal yang sudah lewat: %v; mau 400", err)
	}

	po2 := e.preorder(t, kasir.Id, bolu, "delivery")
	id := e.deliveryOf(t, po2.Id)
	if _, err := deliveries.Fail(e.adminID, id, req.DeliveryReasonRequest{Reason: "rumah kosong"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE deliveries SET scheduled_date = $2 WHERE id = $1`, id, day(-1)); err != nil {
		t.Fatal(err)
	}
	_, err = deliveries.Update(e.adminID, id, req.DeliveryUpdateRequest{ScheduledDate: ptr(day(-1))})
	if code, _ := errOf(err); code != http.StatusBadRequest {
		t.Fatalf("jadwal ulang pengiriman gagal ke tanggal lama: %v; mau 400", err)
	}
	d, err := deliveries.Update(e.adminID, id, req.DeliveryUpdateRequest{ScheduledDate: ptr(day(1))})
	if err != nil || d.Status != model.DeliveryPending || d.ScheduledDate != day(1) {
		t.Fatalf("jadwal ulang ke besok: %v, status %s, tanggal %s", err, d.Status, d.ScheduledDate)
	}
}

// racyDeliveries: tepat setelah usecase membaca pengiriman, orang lain menyimpan perubahannya (during).
type racyDeliveries struct {
	postgres.DeliveryRepository
	during func()
	armed  atomic.Bool
}

func (r *racyDeliveries) FindById(id string) (model.Delivery, error) {
	d, err := r.DeliveryRepository.FindById(id)
	if err == nil && r.armed.CompareAndSwap(true, false) {
		r.during()
	}
	return d, err
}

func TestDeliveryEditCannotUndoDispatch(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	po := e.preorder(t, kasir.Id, bolu, "delivery")
	id := e.deliveryOf(t, po.Id)

	racy := &racyDeliveries{DeliveryRepository: e.repos.DeliveryRepository()}
	racy.during = func() {
		_, err := e.uc.DeliveryUseCase().Dispatch(e.adminID, id, req.DeliveryDispatchRequest{Courier: ptr("Kurir")})
		if err != nil {
			t.Error(err)
		}
	}
	racy.armed.Store(true)
	_, err := usecase.NewDeliveryUseCase(racy).Update(e.adminID, id, req.DeliveryUpdateRequest{Address: ptr("Jl. Lain")})
	if _, reason := errOf(err); reason != "stale" {
		t.Fatalf("ubah alamat yang kalah cepat dari Berangkat: %v; mau stale", err)
	}
	row := one[string](t, e.db, `SELECT status || '|' || address FROM deliveries WHERE id = $1`, id)
	if row != "on_the_way|Jl. Uji 1" {
		t.Fatalf("pengiriman = %s; mau tetap di jalan dengan alamat lama", row)
	}
}

func TestDocNumberWidth(t *testing.T) {
	e := setup(t)
	tx, err := e.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	next := func() string {
		var no string
		if err := tx.QueryRow(`SELECT next_doc_no('UJI', 'uji_doc_seq')`).Scan(&no); err != nil {
			t.Fatal(err)
		}
		return no
	}
	var today string
	if _, err := tx.Exec(`CREATE TEMP SEQUENCE uji_doc_seq START 42`); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(`SELECT to_char(now(), 'YYYYMMDD')`).Scan(&today); err != nil {
		t.Fatal(err)
	}
	if got := next(); got != "UJI-"+today+"-000042" {
		t.Fatalf("nomor biasa = %s; mau 6 digit", got)
	}
	if _, err := tx.Exec(`SELECT setval('uji_doc_seq', 999998)`); err != nil {
		t.Fatal(err)
	}
	got := []string{next(), next()}
	if got[0] != "UJI-"+today+"-999999" || got[1] != "UJI-"+today+"-1000000" {
		t.Fatalf("setelah 999999 = %v; mau lanjut 7 digit, tidak terpotong", got)
	}
}

func TestIntegrityRules(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	teh := e.product(t, req.ProductRequest{Name: "Teh", Price: ptr(int64(5000)), Stock: 10})
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	sale, err := e.uc.TransactionUseCase().Create(kasir.Id, sellOne(teh, 1))
	if err != nil {
		t.Fatal(err)
	}
	voided, _ := e.uc.TransactionUseCase().Create(kasir.Id, sellOne(teh, 1))
	if _, err := e.uc.TransactionUseCase().Void(kasir.Id, model.RoleCashier, voided.Id,
		req.VoidRequest{Reason: "uji"}); err != nil {
		t.Fatal(err)
	}
	po := e.preorder(t, kasir.Id, bolu, "pickup")
	pending := e.deliveryOf(t, e.preorder(t, kasir.Id, bolu, "delivery").Id)

	// Perintah SQL langsung yang melanggar aturan ditolak database, di transaksi yang selalu dibatalkan.
	cases := []struct{ name, query, id, want string }{
		{"total tidak cocok dengan rinciannya", `UPDATE transactions SET total = total + 1 WHERE id = $1`, sale.Id,
			"transactions_total_parts"},
		{"batal tanpa waktu, akun, dan alasan", `UPDATE transactions SET status = 'void' WHERE id = $1`, sale.Id,
			"transactions_void_fields"},
		{"alasan batal dihapus", `UPDATE transactions SET void_reason = NULL WHERE id = $1`, voided.Id,
			"transactions_void_fields"},
		{"PO selesai tanpa struk", `UPDATE preorders SET status = 'completed' WHERE id = $1`, po.Id,
			"preorders_completed_transaction"},
		{"hapus struk yang punya barang", `DELETE FROM transactions WHERE id = $1`, sale.Id,
			"transaction_items_transaction_id_fkey"},
		{"hapus PO yang punya barang dan DP", `DELETE FROM preorders WHERE id = $1`, po.Id, "_preorder_id_fkey"},
		{"penerima di pengiriman yang belum terkirim", `UPDATE deliveries SET received_by = 'x' WHERE id = $1`, pending,
			"deliveries_received_by"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tx, err := e.db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			_, err = tx.Exec(c.query, c.id)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error = %v; mau ditolak %s", err, c.want)
			}
		})
	}
}

func TestReportCountsEachSaleOnce(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	air := e.product(t, req.ProductRequest{Name: "Air", Price: ptr(int64(3000)), Stock: 100})
	today := time.Now().Format("2006-01-02")
	reports := e.uc.ReportUseCase()
	daily := func(source string) res.ReportSummary {
		r, err := reports.Daily(today, today, source)
		if err != nil {
			t.Fatal(err)
		}
		return r.Totals
	}
	before := daily("store")
	for range 2 {
		if _, err := e.uc.TransactionUseCase().Create(kasir.Id, sellOne(air, 2)); err != nil {
			t.Fatal(err)
		}
	}
	after := daily("store")
	if after.TransactionCount-before.TransactionCount != 2 || after.ItemsSold-before.ItemsSold != 4 ||
		after.Revenue-before.Revenue != 12000 {
		t.Fatalf("laporan toko bertambah %d transaksi, %d barang, Rp %d; mau 2, 4, Rp 12000",
			after.TransactionCount-before.TransactionCount, after.ItemsSold-before.ItemsSold,
			after.Revenue-before.Revenue)
	}
	all, po := daily("all"), daily("preorder")
	if all.TransactionCount != after.TransactionCount+po.TransactionCount || all.Revenue != after.Revenue+po.Revenue {
		t.Fatalf("toko + PO harus sama dengan semua: %d + %d vs %d", after.Revenue, po.Revenue, all.Revenue)
	}
}

func TestDailyQuota(t *testing.T) {
	e := setup(t)
	kasir := e.cashier(t)
	bolu := e.product(t, req.ProductRequest{Kind: model.ProductPreorder, Name: "Bolu", Price: ptr(int64(10000))})
	preorders := e.uc.PreorderUseCase()
	// Tanggal jauh dan acak, supaya tidak bertabrakan dengan pesanan uji lain.
	day := time.Now().AddDate(0, 0, 400+int(time.Now().UnixNano()%300)).Format("2006-01-02")
	q, err := preorders.Quota(day, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := preorders.SetDailyLimit(e.adminID, req.PreorderDailyLimitRequest{DailyLimit: q.Used + 5}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = preorders.SetDailyLimit(e.adminID, req.PreorderDailyLimitRequest{DailyLimit: q.DailyLimit})
	})
	order := func(qty int) error {
		_, err := preorders.Create(kasir.Id, req.PreorderRequest{CustomerName: e.name("Bu"), DueDate: day,
			Fulfillment: "pickup", Items: []req.PreorderItemRequest{{ProductId: bolu, Qty: qty}}})
		return err
	}
	if err := order(3); err != nil {
		t.Fatalf("3 dari 5 pcs: %v", err)
	}
	if code, _ := errOf(order(3)); code != http.StatusUnprocessableEntity {
		t.Fatalf("melebihi kuota: status %d; mau 422", code)
	}
	if err := order(2); err != nil {
		t.Fatalf("pas sisa kuota (2 pcs): %v", err)
	}
}

func TestExpiredSessionsRemoved(t *testing.T) {
	e := setup(t)
	ids := map[string]string{}
	for _, n := range []string{"t1", "t2", "t3", "t4", "t5", "t6"} {
		ids[n] = newKey()
	}
	// t1 -> t2 -> t3: rantai rotasi, t3 masih berlaku. t5 masih berlaku tapi penggantinya t6 sudah kedaluwarsa
	// (masa berlaku pernah diperpendek): t6 terhapus dan tautan t5 dikosongkan.
	tokens := []struct {
		id, replacedBy string
		expired        bool
	}{{"t1", "t2", true}, {"t2", "t3", true}, {"t3", "", false}, {"t4", "", true}, {"t6", "", true}, {"t5", "t6", false}}
	for _, tk := range tokens {
		exp := "now() + interval '3 days'"
		if tk.expired {
			exp = "now() - interval '1 hour'"
		}
		if _, err := e.db.Exec(`INSERT INTO refresh_tokens(id, user_id, token_hash, expires_at)
			VALUES ($1, $2, $3, `+exp+`)`, ids[tk.id], e.adminID, "uji-"+ids[tk.id]); err != nil {
			t.Fatal(err)
		}
	}
	for _, tk := range tokens {
		if tk.replacedBy != "" {
			if _, err := e.db.Exec(`UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2 WHERE id = $1`,
				ids[tk.id], ids[tk.replacedBy]); err != nil {
				t.Fatal(err)
			}
		}
	}
	n, err := e.uc.AuthUseCase().DeleteExpiredSessions()
	if err != nil || n < 4 {
		t.Fatalf("dihapus %d, %v; mau minimal 4", n, err)
	}
	left := one[string](t, e.db, `SELECT COALESCE(string_agg(id::text, ','), '') FROM refresh_tokens
		WHERE id = ANY($1::uuid[])`, "{"+strings.Join([]string{ids["t1"], ids["t2"], ids["t3"], ids["t4"], ids["t5"],
		ids["t6"]}, ",")+"}")
	if strings.Count(left, ",") != 1 || !strings.Contains(left, ids["t3"]) || !strings.Contains(left, ids["t5"]) {
		t.Fatalf("tersisa %s; mau hanya t3 dan t5", left)
	}
	if !one[bool](t, e.db, `SELECT replaced_by IS NULL FROM refresh_tokens WHERE id = $1`, ids["t5"]) {
		t.Fatal("tautan ke pengganti yang terhapus harus dikosongkan")
	}
}

func TestOldRequestKeysRemoved(t *testing.T) {
	e := setup(t)
	teh := e.product(t, req.ProductRequest{Name: "Teh", Price: ptr(int64(5000)), Stock: 10})
	oldKey, recentKey := newKey(), newKey()
	for _, k := range []string{oldKey, recentKey} {
		r := sellOne(teh, 1)
		r.RequestKey = k
		if _, err := e.uc.TransactionUseCase().Create(e.adminID, r); err != nil {
			t.Fatal(err)
		}
	}
	// Batasnya 30 hari: kunci berumur 31 hari terhapus, yang 29 hari tetap ada.
	for k, age := range map[string]string{oldKey: "31 days", recentKey: "29 days"} {
		if _, err := e.db.Exec(`UPDATE idempotency_keys SET created_at = now() - $2::interval WHERE request_key = $1`,
			k, age); err != nil {
			t.Fatal(err)
		}
	}
	n, err := e.uc.IdempotencyUseCase().DeleteOldKeys()
	if err != nil || n < 1 {
		t.Fatalf("dihapus %d, %v; mau minimal 1", n, err)
	}
	left := one[string](t, e.db, `SELECT COALESCE(string_agg(request_key::text, ','), '') FROM idempotency_keys
		WHERE request_key = ANY($1::uuid[])`, "{"+oldKey+","+recentKey+"}")
	if left != recentKey {
		t.Fatalf("tersisa %q; mau hanya kunci yang berumur 29 hari", left)
	}
}
