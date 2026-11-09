package usecase

import (
	"net/http"

	"github.com/EkoEdyP/point-of-sale/exception"
)

// reasonStale: kode error saat data yang mau diubah sudah diubah orang lain sejak formulirnya dibuka. Client memuat
// ulang datanya, supaya pengguna melihat perubahan itu sebelum mengulang perubahannya sendiri.
const reasonStale = "stale"

// checkVersion menolak perubahan yang dibuat dari data lama. sent = versi saat formulir dibuka (nil = tidak dikirim,
// tidak dicek), current = versi yang baru dibaca. what = nama datanya di pesan error ("pre-order", "pengiriman").
func checkVersion(sent *int, current int, what string) error {
	if sent == nil || *sent == current {
		return nil
	}
	return staleError(what)
}

func staleError(what string) error {
	return &exception.GetError{
		Code:    http.StatusConflict,
		Message: what + " ini sudah diubah orang lain, muat ulang lalu ulangi",
		Reason:  reasonStale,
	}
}
