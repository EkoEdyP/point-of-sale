package usecase

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
)

// Kuota pre-order: total pcs semua produk PO dengan hari H yang sama tidak boleh melebihi kuota harian.
// Yang dihitung semua pesanan kecuali yang dibatalkan, termasuk yang sudah diserahkan.
// Pengecekan saat menyimpan ada di repository (dalam transaksi yang sama, dengan kunci per tanggal).

// Quota: kuota satu hari H. excludeId (opsional) tidak ikut dihitung: dipakai saat memindah jadwal,
// supaya pesanan yang sedang dipindah tidak terhitung dua kali.
func (u *preorderUseCase) Quota(dueDate, excludeId string) (res.PreorderQuotaResponse, error) {
	d, err := time.Parse(dateLayout, strings.TrimSpace(dueDate))
	if err != nil || d.Year() < 1 {
		return res.PreorderQuotaResponse{}, badRequest("due_date tidak valid, gunakan format YYYY-MM-DD")
	}
	dueDate = d.Format(dateLayout)
	excludeId = strings.ToLower(strings.TrimSpace(excludeId))
	if err := u.validate.Var(excludeId, "omitempty,uuid"); err != nil {
		return res.PreorderQuotaResponse{}, badRequest("exclude tidak valid")
	}

	limit, err := u.preorders.DailyLimit()
	if err != nil {
		return res.PreorderQuotaResponse{}, internalError(err)
	}
	used, err := u.preorders.QuotaUsed([]string{dueDate}, excludeId)
	if err != nil {
		return res.PreorderQuotaResponse{}, internalError(err)
	}
	return res.PreorderQuotaResponse{
		DueDate:    dueDate,
		DailyLimit: limit,
		Used:       used[dueDate],
		Remaining:  max(limit-used[dueDate], 0),
	}, nil
}

// SetDailyLimit mengubah kuota per hari (hanya admin, dibatasi di controller). Pesanan yang sudah ada
// tidak berubah; kuota baru berlaku untuk pesanan baru dan pemindahan jadwal.
func (u *preorderUseCase) SetDailyLimit(
	actorId string, request req.PreorderDailyLimitRequest,
) (res.PreorderDailyLimitResponse, error) {
	if err := u.validate.Struct(request); err != nil {
		return res.PreorderDailyLimitResponse{}, badRequest("kuota per hari harus 1 sampai 1.000.000 pcs")
	}
	if err := u.preorders.SetDailyLimit(request.DailyLimit, actorId); err != nil {
		return res.PreorderDailyLimitResponse{}, internalError(err)
	}
	return res.PreorderDailyLimitResponse{DailyLimit: request.DailyLimit}, nil
}

// quotaExceeded: pesan untuk kasir saat pesanan tidak muat di kuota hari H.
// "sudah dipesan" bisa lebih besar dari kuota kalau admin menurunkan kuota setelah ada pesanan.
func quotaExceeded(e *postgres.QuotaExceededError) error {
	date := longDate(e.DueDate)
	remaining := max(e.Limit-e.Used, 0)
	if remaining == 0 {
		return unprocessable(fmt.Sprintf(
			"Kuota pre-order %s sudah penuh (kuota %s pcs, sudah dipesan %s pcs). Pilih tanggal lain.",
			date, groupThousands(e.Limit), groupThousands(e.Used),
		))
	}
	return unprocessable(fmt.Sprintf(
		"Kuota pre-order %s tinggal %s pcs (kuota %s pcs, sudah dipesan %s pcs), sedangkan pesanan ini %s pcs. "+
			"Kurangi jumlah atau pilih tanggal lain.",
		date, groupThousands(remaining), groupThousands(e.Limit), groupThousands(e.Used), groupThousands(e.Requested),
	))
}

var (
	weekdayNames = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	monthNames   = [...]string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
)

// longDate: "2026-10-30" menjadi "Jumat, 30 Oktober 2026".
func longDate(date string) string {
	d, err := time.Parse(dateLayout, date)
	if err != nil {
		return date
	}
	return fmt.Sprintf("%s, %d %s %d", weekdayNames[d.Weekday()], d.Day(), monthNames[d.Month()-1], d.Year())
}

// groupThousands: 12500 menjadi "12.500".
func groupThousands(n int) string {
	s := strconv.Itoa(n)
	sign := ""
	if n < 0 {
		sign, s = "-", s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return sign + b.String()
}
