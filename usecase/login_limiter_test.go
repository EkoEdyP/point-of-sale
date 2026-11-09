package usecase

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/EkoEdyP/point-of-sale/exception"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter() (*loginLimiter, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)}
	return newLoginLimiter(clock.now), clock
}

// attempt menjalankan satu percobaan penuh: begin lalu done. Mengembalikan lama tunggu kalau ditolak.
func attempt(l *loginLimiter, key string, outcome loginOutcome) time.Duration {
	wait, reserved := l.begin(key)
	if wait > 0 {
		return wait
	}
	l.done(key, reserved, outcome, true)
	return 0
}

func TestLoginLimiterBlocksAfterFiveFailuresAndDoubles(t *testing.T) {
	l, clock := newTestLimiter()
	for i := 1; i <= loginFreeFailures; i++ {
		if wait := attempt(l, "budi", loginFailed); wait != 0 {
			t.Fatalf("gagal ke-%d sudah ditolak (%v), seharusnya masih bebas", i, wait)
		}
	}
	if wait, _ := l.begin("budi"); wait != 30*time.Second {
		t.Fatalf("setelah 5 gagal: tunggu %v, mau 30s", wait)
	}
	// Percobaan selama menunggu tidak memperpanjang tunggu.
	clock.advance(20 * time.Second)
	if wait, _ := l.begin("budi"); wait != 10*time.Second {
		t.Fatalf("20 detik kemudian: tunggu %v, mau 10s", wait)
	}

	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 15 * time.Minute,
		15 * time.Minute}
	for i, w := range want {
		clock.advance(loginMaxBlock)
		if wait := attempt(l, "budi", loginFailed); wait != 0 {
			t.Fatalf("setelah jeda habis, percobaan ke-%d ditolak (%v)", i+6, wait)
		}
		if wait, _ := l.begin("budi"); wait != w {
			t.Fatalf("gagal ke-%d: tunggu %v, mau %v", i+6, wait, w)
		}
	}
}

func TestLoginLimiterSuccessResets(t *testing.T) {
	l, clock := newTestLimiter()
	for range loginFreeFailures {
		attempt(l, "edy", loginFailed)
	}
	clock.advance(loginFirstBlock)
	if wait := attempt(l, "edy", loginSucceeded); wait != 0 {
		t.Fatalf("login benar setelah jeda ditolak (%v)", wait)
	}
	if len(l.entries) != 0 {
		t.Fatalf("hitungan tidak terhapus setelah berhasil: %d entri", len(l.entries))
	}
	for i := 1; i <= loginFreeFailures; i++ {
		if wait := attempt(l, "edy", loginFailed); wait != 0 {
			t.Fatalf("setelah berhasil, gagal ke-%d sudah ditolak (%v)", i, wait)
		}
	}
}

func TestLoginLimiterForgetsAfterQuietPeriod(t *testing.T) {
	l, clock := newTestLimiter()
	for range loginFreeFailures - 1 {
		attempt(l, "admin", loginFailed)
	}
	clock.advance(loginForgetAfter)
	for i := 1; i <= loginFreeFailures; i++ {
		if wait := attempt(l, "admin", loginFailed); wait != 0 {
			t.Fatalf("30 menit tanpa gagal: hitungan lama masih dipakai, gagal ke-%d ditolak (%v)", i, wait)
		}
	}
}

func TestLoginLimiterNeutralNotCounted(t *testing.T) {
	l, _ := newTestLimiter()
	for range 20 {
		attempt(l, "budi", loginNeutral)
	}
	if len(l.entries) != 0 {
		t.Fatalf("error server ikut dihitung: %d entri", len(l.entries))
	}
}

// Tebakan yang dikirim bersamaan tidak boleh melewati jatah bebas sebelum gagalnya tercatat.
func TestLoginLimiterConcurrentAttempts(t *testing.T) {
	l, _ := newTestLimiter()
	var reservedCount int
	var mu sync.Mutex
	var wg sync.WaitGroup
	start := make(chan struct{})
	reservations := make(chan bool, 50)
	for range 50 {
		wg.Go(func() {
			<-start
			wait, reserved := l.begin("budi")
			if wait == 0 {
				mu.Lock()
				reservedCount++
				mu.Unlock()
			}
			reservations <- reserved
		})
	}
	close(start)
	wg.Wait()
	close(reservations)
	if reservedCount != loginFreeFailures {
		t.Fatalf("%d percobaan bersamaan lolos, mau %d", reservedCount, loginFreeFailures)
	}
	for reserved := range reservations {
		if reserved {
			l.done("budi", true, loginFailed, true)
		}
	}
	if wait, _ := l.begin("budi"); wait != loginFirstBlock {
		t.Fatalf("setelah 5 tebakan bersamaan gagal: tunggu %v, mau %v", wait, loginFirstBlock)
	}

	// Setelah jeda pertama hanya satu percobaan sekaligus.
	l2, clock := newTestLimiter()
	for range loginFreeFailures {
		attempt(l2, "edy", loginFailed)
	}
	clock.advance(loginFirstBlock)
	if wait, _ := l2.begin("edy"); wait != 0 {
		t.Fatalf("percobaan pertama setelah jeda ditolak (%v)", wait)
	}
	if wait, _ := l2.begin("edy"); wait == 0 {
		t.Fatal("percobaan kedua selagi yang pertama diperiksa ikut lolos")
	}
}

func TestLoginLimiterFullMapStillTracksKnownUsers(t *testing.T) {
	l, _ := newTestLimiter()
	for i := range loginMaxEntries {
		l.entries[fmt.Sprintf("acak-%d", i)] = &loginEntry{failures: 1, lastFailure: l.now()}
	}
	// Username yang tidak ada tidak dicatat lagi, username yang ada tetap dicatat.
	if wait := attempt(l, "acak", loginFailed); wait != 0 {
		t.Fatalf("username baru saat penuh ditolak (%v)", wait)
	}
	wait, reserved := l.begin("acak2")
	l.done("acak2", reserved, loginFailed, false)
	if _, ok := l.entries["acak2"]; ok || wait != 0 {
		t.Fatal("username yang tidak ada ikut dicatat saat penuh")
	}
	for range loginFreeFailures {
		wait, reserved := l.begin("budi")
		if wait != 0 {
			break
		}
		l.done("budi", reserved, loginFailed, true)
	}
	if wait, _ := l.begin("budi"); wait != loginFirstBlock {
		t.Fatalf("username yang ada tidak dibatasi saat penuh: tunggu %v", wait)
	}
}

func TestLoginKeyIgnoresCaseAndLength(t *testing.T) {
	if loginKey("BuDi") != "budi" {
		t.Fatal("huruf besar menghasilkan kunci lain")
	}
	if got := loginKey(strings.Repeat("a", 500)); len(got) != loginKeyMaxLen {
		t.Fatalf("kunci tidak dipotong: %d karakter", len(got))
	}
}

func TestLoginThrottledMessage(t *testing.T) {
	cases := []struct {
		wait  time.Duration
		text  string
		retry int
	}{
		{30 * time.Second, "30 detik", 30},
		{1500 * time.Millisecond, "2 detik", 2},
		{time.Minute, "60 detik", 60},
		{4 * time.Minute, "4 menit", 240},
		{61 * time.Second, "2 menit", 61},
	}
	for _, c := range cases {
		err := loginThrottled(c.wait)
		ge, ok := err.(*exception.GetError)
		if !ok || ge.Code != 429 || ge.RetryAfter != c.retry || !strings.HasSuffix(ge.Message, c.text) {
			t.Fatalf("%v: dapat %#v, mau akhiran %q dan Retry-After %d", c.wait, err, c.text, c.retry)
		}
	}
}
