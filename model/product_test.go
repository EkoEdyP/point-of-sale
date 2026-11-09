package model

import "testing"

func TestUnitPrice(t *testing.T) {
	wholesale := int64(4000)
	p := Product{Price: 5000, WholesalePrice: &wholesale, WholesaleMinQty: 50}
	for qty, want := range map[int]int64{1: 5000, 49: 5000, 50: 4000, 500: 4000} {
		if got := p.UnitPrice(qty); got != want {
			t.Errorf("UnitPrice(%d) = %d, mau %d", qty, got, want)
		}
	}
	plain := Product{Price: 5000, WholesaleMinQty: 50}
	if got := plain.UnitPrice(100); got != 5000 {
		t.Errorf("tanpa harga grosir: UnitPrice(100) = %d, mau 5000", got)
	}
}
