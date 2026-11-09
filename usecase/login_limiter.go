package usecase

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/EkoEdyP/point-of-sale/exception"
)

// Pembatas tebakan password. Kuncinya username (huruf kecil), bukan IP: di belakang proxy Vite semua request datang
// dari 127.0.0.1. Lima gagal berturut-turut masih bebas; setelah itu akun itu harus menunggu, mulai 30 detik dan
// berlipat dua setiap gagal lagi, paling lama 15 menit. Login berhasil, atau 30 menit tanpa gagal, menghapus
// hitungannya. Percobaan selama menunggu ditolak tanpa memeriksa password dan tidak memperpanjang tunggu, jadi
// pemilik akun paling lama menunggu satu jeda. Hitungan ada di memori dan hilang saat server di-restart.
const (
	loginFreeFailures = 5
	loginFirstBlock   = 30 * time.Second
	loginMaxBlock     = 15 * time.Minute
	loginForgetAfter  = 30 * time.Minute
	// Batas jumlah username yang dicatat, supaya username acak tidak menghabiskan memori. Saat penuh, username
	// baru yang tidak ada di database tidak dicatat; username yang ada selalu dicatat.
	loginMaxEntries = 10000
	loginKeyMaxLen  = 64
)

type loginOutcome int

const (
	loginSucceeded loginOutcome = iota
	loginFailed
	loginNeutral // bukan soal password (misalnya database error): tidak dihitung
)

type loginLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]*loginEntry
}

type loginEntry struct {
	failures     int // gagal berturut-turut
	pending      int // percobaan yang password-nya sedang diperiksa
	lastFailure  time.Time
	blockedUntil time.Time
}

// logins dipakai bersama oleh semua authUseCase di proses ini (manager membuat use case baru di setiap panggilan).
var logins = newLoginLimiter(time.Now)

func newLoginLimiter(now func() time.Time) *loginLimiter {
	return &loginLimiter{now: now, entries: map[string]*loginEntry{}}
}

func loginKey(username string) string {
	key := strings.ToLower(username)
	if len(key) > loginKeyMaxLen {
		key = key[:loginKeyMaxLen]
	}
	return key
}

// begin memesan satu percobaan login. wait > 0 berarti ditolak dan harus menunggu selama itu. reserved diteruskan
// ke done.
func (l *loginLimiter) begin(key string) (wait time.Duration, reserved bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	e := l.entries[key]
	if e == nil {
		if len(l.entries) >= loginMaxEntries {
			l.prune(now)
		}
		if len(l.entries) >= loginMaxEntries {
			return 0, false
		}
		e = &loginEntry{}
		l.entries[key] = e
	}
	if now.Before(e.blockedUntil) {
		return e.blockedUntil.Sub(now), false
	}
	if e.pending == 0 && e.failures > 0 && now.Sub(e.lastFailure) >= loginForgetAfter {
		e.failures = 0
	}
	// Sebelum jeda pertama boleh beberapa percobaan sekaligus selama jumlahnya tidak melewati jatah bebas. Setelah
	// itu satu per satu, supaya tebakan yang dikirim bersamaan tidak lolos sebelum gagalnya tercatat.
	if e.failures < loginFreeFailures && e.failures+e.pending >= loginFreeFailures ||
		e.failures >= loginFreeFailures && e.pending > 0 {
		return time.Second, false
	}
	e.pending++
	return 0, true
}

// done mencatat hasil percobaan dari begin. known: username itu ada di database.
func (l *loginLimiter) done(key string, reserved bool, outcome loginOutcome, known bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	e := l.entries[key]
	if e == nil {
		if outcome != loginFailed || !known {
			return
		}
		e = &loginEntry{}
		l.entries[key] = e
	}
	if reserved && e.pending > 0 {
		e.pending--
	}
	switch outcome {
	case loginSucceeded:
		e.failures = 0
		e.blockedUntil = time.Time{}
	case loginFailed:
		e.failures++
		e.lastFailure = now
		if e.failures >= loginFreeFailures {
			e.blockedUntil = now.Add(loginBlockFor(e.failures))
		}
	}
	if e.failures == 0 && e.pending == 0 {
		delete(l.entries, key)
	}
}

// prune membuang hitungan yang sudah kedaluwarsa. Dipanggil dengan mu terkunci.
func (l *loginLimiter) prune(now time.Time) {
	for key, e := range l.entries {
		if e.pending == 0 && !now.Before(e.blockedUntil) && now.Sub(e.lastFailure) >= loginForgetAfter {
			delete(l.entries, key)
		}
	}
}

// loginBlockFor: lama tunggu setelah gagal ke-n (n >= loginFreeFailures).
func loginBlockFor(failures int) time.Duration {
	d := loginFirstBlock
	for i := loginFreeFailures; i < failures && d < loginMaxBlock; i++ {
		d *= 2
	}
	return min(d, loginMaxBlock)
}

func loginThrottled(wait time.Duration) error {
	seconds := int(math.Ceil(wait.Seconds()))
	text := fmt.Sprintf("%d detik", seconds)
	if wait > time.Minute {
		text = fmt.Sprintf("%d menit", int(math.Ceil(wait.Minutes())))
	}
	return &exception.GetError{
		Code:       http.StatusTooManyRequests,
		Message:    "terlalu banyak percobaan login yang gagal, coba lagi dalam " + text,
		RetryAfter: seconds,
	}
}
