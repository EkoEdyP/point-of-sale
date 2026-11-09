package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
)

func TestIsBusy(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"statement_timeout":           {&pq.Error{Code: "57014"}, true},
		"lock_timeout":                {&pq.Error{Code: "55P03"}, true},
		"deadlock":                    {&pq.Error{Code: "40P01"}, true},
		"koneksi penuh":               {&pq.Error{Code: "53300"}, true},
		"dibungkus fmt.Errorf":        {fmt.Errorf("simpan: %w", &pq.Error{Code: "57014"}), true},
		"data duplikat (bukan sibuk)": {&pq.Error{Code: "23505"}, false},
		"error biasa":                 {errors.New("x"), false},
		"nil":                         {nil, false},
	}
	for name, c := range cases {
		if got := IsBusy(c.err); got != c.want {
			t.Errorf("%s: IsBusy = %v, mau %v", name, got, c.want)
		}
	}
}
