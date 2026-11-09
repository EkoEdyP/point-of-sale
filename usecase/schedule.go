package usecase

import (
	"strings"
	"time"
)

// checkScheduleDate: aturan yang sama untuk hari H pre-order dan tanggal kirim pengiriman. value harus YYYY-MM-DD
// dan tidak sebelum hari ini (menurut jam server). current = tanggal yang sudah tersimpan, kosong untuk data baru atau
// penjadwalan ulang: tanggal yang tidak berubah tidak dicek lagi, supaya data yang jadwalnya sudah lewat tetap bisa
// diubah jam atau catatannya. field = nama field di request, label = nama tanggalnya di pesan untuk pengguna.
func checkScheduleDate(value, current, field, label string) (string, error) {
	d, err := time.Parse(dateLayout, strings.TrimSpace(value))
	if err != nil {
		return "", badRequest(field + " tidak valid, gunakan format YYYY-MM-DD")
	}
	date := d.Format(dateLayout)
	if date != current && date < time.Now().Format(dateLayout) {
		return "", badRequest(label + " tidak boleh sebelum hari ini")
	}
	return date, nil
}
