package usecase

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/EkoEdyP/point-of-sale/exception"
)

// wantError memastikan err adalah error usecase dengan status dan pesan yang diharapkan.
func wantError(t *testing.T, err error, status int, message string) {
	t.Helper()
	var ge *exception.GetError
	if !errors.As(err, &ge) {
		t.Fatalf("error = %v, mau *exception.GetError %d %q", err, status, message)
	}
	if ge.Code != status || ge.Message != message {
		t.Fatalf("error = %d %q, mau %d %q", ge.Code, ge.Message, status, message)
	}
}

func TestCheckScheduleDate(t *testing.T) {
	day := func(offset int) string { return time.Now().AddDate(0, 0, offset).Format(dateLayout) }
	today := day(0)
	cases := []struct {
		name, value, current, want, wantErr string
	}{
		{name: "hari ini", value: today, want: today},
		{name: "besok, spasi di tepi dibuang", value: " " + day(1) + " ", want: day(1)},
		{name: "kemarin untuk data baru", value: day(-1), wantErr: "tanggal kirim tidak boleh sebelum hari ini"},
		{name: "tanggal lama yang tidak berubah tetap boleh", value: day(-3), current: day(-3), want: day(-3)},
		{name: "pindah ke tanggal lain yang sudah lewat", value: day(-2), current: day(-3),
			wantErr: "tanggal kirim tidak boleh sebelum hari ini"},
		{name: "format salah", value: "05/11/2026", wantErr: "scheduled_date tidak valid, gunakan format YYYY-MM-DD"},
		{name: "tahun 0000 (ditolak Postgres)", value: "0000-01-01",
			wantErr: "tanggal kirim tidak boleh sebelum hari ini"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := checkScheduleDate(c.value, c.current, "scheduled_date", "tanggal kirim")
			if c.wantErr != "" {
				wantError(t, err, http.StatusBadRequest, c.wantErr)
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("checkScheduleDate(%q, %q) = %q, %v; mau %q", c.value, c.current, got, err, c.want)
			}
		})
	}
	if day(0) != today {
		t.Skip("tanggal berganti saat tes berjalan")
	}
}

func TestCheckVersion(t *testing.T) {
	one, two := 1, 2
	if err := checkVersion(nil, 5, "pre-order"); err != nil {
		t.Fatalf("versi tidak dikirim (skrip, client lama) harus diterima: %v", err)
	}
	if err := checkVersion(&two, 2, "pre-order"); err != nil {
		t.Fatalf("versi yang sama harus diterima: %v", err)
	}
	err := checkVersion(&one, 2, "pengiriman")
	wantError(t, err, http.StatusConflict, "pengiriman ini sudah diubah orang lain, muat ulang lalu ulangi")
	var ge *exception.GetError
	if errors.As(err, &ge); ge.Reason != reasonStale {
		t.Fatalf("Reason = %q, mau %q (client mengenali kode ini)", ge.Reason, reasonStale)
	}
}
