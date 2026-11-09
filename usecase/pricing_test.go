package usecase

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/EkoEdyP/point-of-sale/exception"
)

func TestMergeItems(t *testing.T) {
	a := "6f1b0c3e-0000-4000-8000-00000000000a"
	b := "6f1b0c3e-0000-4000-8000-00000000000b"
	got, err := mergeItems([]itemQty{{b, 2}, {strings.ToUpper(a), 1}, {a, 4}, {b, 1}})
	if err != nil {
		t.Fatal(err)
	}
	// Produk yang sama digabung (huruf besar/kecil id tidak dibedakan) lalu diurutkan per id.
	want := []itemQty{{a, 5}, {b, 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeItems = %v, mau %v", got, want)
	}

	_, err = mergeItems([]itemQty{{a, 6000}, {a, 4001}})
	wantError(t, err, http.StatusUnprocessableEntity, "jumlah per produk maksimal 10000")
}

func TestCheckExpectedTotal(t *testing.T) {
	same, other := int64(15000), int64(16000)
	if err := checkExpectedTotal(nil, 15000); err != nil {
		t.Fatalf("expected_total tidak dikirim harus diterima: %v", err)
	}
	if err := checkExpectedTotal(&same, 15000); err != nil {
		t.Fatalf("total sama harus diterima: %v", err)
	}
	err := checkExpectedTotal(&other, 15000)
	wantError(t, err, http.StatusConflict, "harga berubah: total sekarang Rp 15.000, bukan Rp 16.000")
	var ge *exception.GetError
	if errors.As(err, &ge); ge.Reason != reasonPriceChanged {
		t.Fatalf("Reason = %q, mau %q", ge.Reason, reasonPriceChanged)
	}
}

func TestGroupThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1.000", 1234567: "1.234.567", -5000: "-5.000"} {
		if got := groupThousands(n); got != want {
			t.Errorf("groupThousands(%d) = %q, mau %q", n, got, want)
		}
	}
}

func TestRequestKey(t *testing.T) {
	payload := map[string]any{"items": []string{"a"}, "method": "cash"}
	k, err := requestKey("", "u1", actionSale, payload)
	if k != nil || err != nil {
		t.Fatalf("tanpa header: %v, %v; mau nil, nil (diproses seperti biasa)", k, err)
	}
	_, err = requestKey("abc", "u1", actionSale, payload)
	wantError(t, err, http.StatusBadRequest, "Idempotency-Key tidak valid (harus UUID)")

	raw := "7C9E6679-7425-40DE-944B-E07FC1F90AE7"
	k1, err := requestKey(raw, "u1", actionSale, payload)
	if err != nil {
		t.Fatal(err)
	}
	if k1.Key != strings.ToLower(raw) || k1.UserId != "u1" || k1.Action != actionSale {
		t.Fatalf("kunci = %+v", k1)
	}
	k2, _ := requestKey(strings.ToLower(raw), "u1", actionSale, map[string]any{"method": "cash", "items": []string{"a"}})
	if k2.Hash != k1.Hash {
		t.Fatal("isi yang sama harus menghasilkan sidik yang sama")
	}
	k3, _ := requestKey(raw, "u1", actionSale, map[string]any{"items": []string{"b"}, "method": "cash"})
	if k3.Hash == k1.Hash {
		t.Fatal("isi yang berbeda harus menghasilkan sidik yang berbeda")
	}
}
