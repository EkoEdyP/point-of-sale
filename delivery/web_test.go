package delivery

import (
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/gofiber/fiber/v3"
)

const testIndex = "<!doctype html><title>FN Boss</title>"

// newWebTestApp menyusun route seperti server: API dulu, lalu tampilan web untuk semua alamat GET yang tersisa.
func newWebTestApp(t *testing.T, fsys fs.FS) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: middleware.NewErrorHandler()})
	app.Get("/api/v1/health", func(c fiber.Ctx) error { return c.SendString("ok") })
	handler, err := webHandler(fsys)
	if err != nil {
		t.Fatal(err)
	}
	app.Get("/*", handler)
	return app
}

func get(t *testing.T, app *fiber.App, target string) (status int, body string, header func(string) string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, target, nil))
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(raw), resp.Header.Get
}

func TestWebHandler(t *testing.T) {
	app := newWebTestApp(t, fstest.MapFS{
		"index.html":              {Data: []byte(testIndex)},
		"assets/index-abc123.js":  {Data: []byte("console.log(1)")},
		"assets/index-abc123.css": {Data: []byte("body{}")},
		"manifest.webmanifest":    {Data: []byte(`{"name":"FN Boss"}`)},
		"icons/icon-192.png":      {Data: []byte("\x89PNG\r\n")},
	})
	tests := []struct {
		name, target string
		status       int
		body         string
		contentType  string
		cache        string
	}{
		{"beranda", "/", 200, testIndex, "text/html", cacheRevalidate},
		{"halaman aplikasi", "/kasir", 200, testIndex, "text/html", cacheRevalidate},
		{"halaman dengan query", "/preorder?lihat=produksi", 200, testIndex, "text/html", cacheRevalidate},
		{"aset JS", "/assets/index-abc123.js", 200, "console.log(1)", "text/javascript", cacheImmutable},
		{"aset CSS", "/assets/index-abc123.css", 200, "body{}", "text/css", cacheImmutable},
		{"manifest", "/manifest.webmanifest", 200, "FN Boss", "application/manifest+json", cacheRevalidate},
		{"ikon", "/icons/icon-192.png", 200, "PNG", "image/png", cacheRevalidate},
		{"aset dari build lama", "/assets/index-lama.js", 404, `"error"`, "application/json", ""},
		{"API tetap jalan", "/api/v1/health", 200, "ok", "", ""},
		{"API salah alamat tetap JSON", "/api/v1/tidak-ada", 404, `"error"`, "application/json", ""},
		{"/api tanpa garis miring", "/api", 404, `"error"`, "application/json", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, body, header := get(t, app, tc.target)
			if status != tc.status || !strings.Contains(body, tc.body) {
				t.Fatalf("status %d, isi %q; mau %d berisi %q", status, body, tc.status, tc.body)
			}
			if ct := header(fiber.HeaderContentType); tc.contentType != "" && !strings.HasPrefix(ct, tc.contentType) {
				t.Errorf("Content-Type %q, mau %q", ct, tc.contentType)
			}
			if cc := header(fiber.HeaderCacheControl); tc.cache != "" && cc != tc.cache {
				t.Errorf("Cache-Control %q, mau %q", cc, tc.cache)
			}
		})
	}
}

// Alamat yang mencoba keluar dari folder web tidak boleh membaca file di luarnya, misalnya env.sh di folder proyek.
func TestWebHandlerStaysInsideDir(t *testing.T) {
	root := t.TempDir()
	dist := filepath.Join(root, "dist")
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(testIndex), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "env.sh"), []byte("RAHASIA"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := newWebTestApp(t, os.DirFS(dist))
	for _, target := range []string{
		"/../env.sh",
		"/..%2fenv.sh",
		"/%2e%2e/env.sh",
		"/%2e%2e%2fenv.sh",
		"/assets/..%2f..%2fenv.sh",
		"/..%5cenv.sh",
		"/assets/%2e%2e%5c%2e%2e%5cenv.sh",
		"/assets/..\\..\\env.sh",
	} {
		status, body, _ := get(t, app, target)
		t.Logf("GET %s -> %d", target, status)
		if strings.Contains(body, "RAHASIA") {
			t.Errorf("GET %s membaca file di luar folder web (status %d)", target, status)
		}
	}
}

func TestAppURL(t *testing.T) {
	tests := []struct{ host, port, want string }{
		{"localhost", "8080", "http://localhost:8080/"},
		{"0.0.0.0", "8080", "http://localhost:8080/"},
		{"", "8080", "http://localhost:8080/"},
		{"::", "8080", "http://localhost:8080/"},
		{"[::]", "8080", "http://localhost:8080/"},
		{"::1", "8080", "http://[::1]:8080/"},
		{"192.168.1.10", "9000", "http://192.168.1.10:9000/"},
	}
	for _, tc := range tests {
		if got := appURL(tc.host, tc.port); got != tc.want {
			t.Errorf("appURL(%q, %q) = %q, mau %q", tc.host, tc.port, got, tc.want)
		}
	}
}

func TestPrivateURLs(t *testing.T) {
	addr := func(cidr string) net.Addr {
		ip, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatal(err)
		}
		ipnet.IP = ip
		return ipnet
	}
	got := privateURLs([]net.Addr{
		addr("127.0.0.1/8"),
		addr("192.168.1.10/24"),
		addr("10.0.0.5/8"),
		addr("172.20.3.4/16"),
		addr("8.8.8.8/32"),
		addr("169.254.10.1/16"),
		addr("fe80::1/64"),
		addr("fd00::5/64"),
	}, "8080")
	want := "http://192.168.1.10:8080/ http://10.0.0.5:8080/ http://172.20.3.4:8080/"
	if strings.Join(got, " ") != want {
		t.Errorf("%q, mau %q", got, want)
	}
	if lanURLs("localhost", "8080") != nil || lanURLs("127.0.0.1", "8080") != nil {
		t.Error("server yang hanya mendengarkan satu alamat tidak punya alamat Wi-Fi")
	}
}

func TestPOSAnswers(t *testing.T) {
	pos := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer pos.Close()
	other := httptest.NewServer(http.NotFoundHandler())
	defer other.Close()
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL + "/"
	closed.Close()

	if !posAnswers(pos.URL + "/") {
		t.Error("server POS yang berjalan tidak dikenali")
	}
	if posAnswers(other.URL + "/") {
		t.Error("program lain di port yang sama dianggap POS")
	}
	if posAnswers(closedURL) {
		t.Error("port kosong dianggap POS")
	}
}

func TestBrowserCommand(t *testing.T) {
	const url = "http://localhost:8080/"
	tests := map[string]string{
		"windows": "rundll32 url.dll,FileProtocolHandler " + url,
		"darwin":  "open " + url,
		"linux":   "xdg-open " + url,
	}
	for goos, want := range tests {
		if got := strings.Join(browserCommand(goos, url).Args, " "); got != want {
			t.Errorf("%s: %q, mau %q", goos, got, want)
		}
	}
}
