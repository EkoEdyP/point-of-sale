package postgres

import (
	"errors"

	"github.com/lib/pq"
)

// ErrStale: baris yang mau diubah sudah diubah orang lain sejak versinya dibaca. Perubahannya ditolak, supaya data
// lama di formulir tidak menimpa perubahan itu.
var ErrStale = errors.New("row changed since it was read")

// IsBusy: database menolak sementara, bukan karena datanya salah. Query dibatalkan karena statement_timeout
// (57014), menunggu baris yang dikunci transaksi lain melewati lock_timeout (55P03), deadlock (40P01), atau
// koneksi ke Postgres sudah penuh (53300). Request yang sama biasanya berhasil kalau dicoba lagi.
func IsBusy(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	switch pqErr.Code {
	case "57014", "55P03", "40P01", "53300":
		return true
	}
	return false
}
