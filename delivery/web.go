package delivery

import (
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

// webMimeTypes menimpa tipe isi file tampilan web. Di Windows, paket mime membaca tipe dari registry, dan di sebagian
// PC .js terdaftar sebagai text/plain, sehingga browser menolak menjalankan aplikasinya.
var webMimeTypes = map[string]string{
	".css":         "text/css; charset=utf-8",
	".html":        "text/html; charset=utf-8",
	".ico":         "image/x-icon",
	".jpg":         "image/jpeg",
	".js":          "text/javascript; charset=utf-8",
	".json":        "application/json",
	".png":         "image/png",
	".svg":         "image/svg+xml",
	".webmanifest": "application/manifest+json",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
}

// File di assets/ diberi nama dengan hash isinya oleh Vite, jadi boleh disimpan browser selamanya. File lain
// (index.html, ikon, manifest) dicek ulang setiap dibuka, supaya hasil build baru langsung terpakai.
const (
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
)

// initWeb menyajikan hasil build tampilan web (npm run build) dari Cfg.WebDir, supaya PC kasir cukup menjalankan satu
// program, tanpa server Vite. Folder tanpa index.html dilewati: saat development, Vite yang menyajikan tampilannya.
// Dipanggil setelah semua route API, karena route ini menangkap semua alamat GET yang tersisa.
func (s *Server) initWeb() {
	dir := s.Cfg.ApiConfig.WebDir
	fsys := os.DirFS(dir)
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		s.Log.WithField("dir", dir).Info("tampilan web tidak disajikan: index.html belum ada (npm run build di web/)")
		return
	}
	handler, err := webHandler(fsys)
	if err != nil {
		s.Log.WithError(err).Error("tampilan web tidak disajikan")
		return
	}
	s.Engine.Get("/*", handler)
	s.Log.WithField("dir", dir).Info("tampilan web disajikan")
}

// webHandler menyajikan file dari fsys. Alamat halaman aplikasi (/kasir, /preorder, ...) bukan file, jadi dijawab
// dengan index.html, lalu React yang memilih halamannya. File yang tidak ada, misalnya aset dari build lama, tetap
// 404. Alamat /api tidak pernah dijawab dengan halaman, supaya request API yang salah alamat tetap mendapat error JSON.
func webHandler(fsys fs.FS) (fiber.Handler, error) {
	for ext, typ := range webMimeTypes {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			return nil, fmt.Errorf("tipe file %s: %w", ext, err)
		}
	}
	return static.New("", static.Config{
		FS:   fsys,
		Next: func(c fiber.Ctx) bool { return isAPIPath(c.Path()) },
		ModifyResponse: func(c fiber.Ctx) error {
			c.Set(fiber.HeaderCacheControl, cacheControl(c.Path()))
			return nil
		},
		NotFoundHandler: func(c fiber.Ctx) error {
			if path.Ext(c.Path()) != "" {
				return fiber.ErrNotFound
			}
			c.Set(fiber.HeaderCacheControl, cacheRevalidate)
			return c.Status(fiber.StatusOK).SendFile("index.html", fiber.SendFile{FS: fsys})
		},
	}), nil
}

func isAPIPath(p string) bool {
	return p == "/api" || strings.HasPrefix(p, "/api/")
}

func cacheControl(p string) string {
	if strings.HasPrefix(p, "/assets/") {
		return cacheImmutable
	}
	return cacheRevalidate
}

// appURL: alamat aplikasi dari PC ini. Server yang mendengarkan semua alamat (0.0.0.0, ::) dibuka lewat localhost.
func appURL(host, port string) string {
	host = strings.Trim(host, "[]")
	if listensOnAll(host) {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + "/"
}

func listensOnAll(host string) bool {
	switch strings.Trim(host, "[]") {
	case "", "0.0.0.0", "::":
		return true
	}
	return false
}

// lanURLs: alamat untuk HP dan tablet di jaringan toko, yaitu alamat IPv4 privat PC ini. Kosong kalau server hanya
// mendengarkan satu alamat (APP_API_HOST=localhost), karena perangkat lain memang tidak bisa membukanya.
func lanURLs(host, port string) []string {
	if !listensOnAll(host) {
		return nil
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	return privateURLs(addrs, port)
}

func privateURLs(addrs []net.Addr, port string) []string {
	var urls []string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ip := ipnet.IP.To4(); ip != nil && ip.IsPrivate() {
			urls = append(urls, "http://"+net.JoinHostPort(ip.String(), port)+"/")
		}
	}
	return urls
}

// printAddresses menulis di jendela start.bat alamat aplikasinya dan bahwa server berjalan selama jendela itu terbuka.
func printAddresses(url string, lan []string) {
	fmt.Printf("POS berjalan di %s\n", url)
	for _, u := range lan {
		fmt.Printf("Dari HP atau tablet di Wi-Fi toko: %s\n", u)
	}
	fmt.Println("Tutup jendela ini untuk mematikan server.")
}

// posAnswers: apakah server POS sudah menjawab di url, misalnya dari jendela start.bat yang masih terbuka.
func posAnswers(url string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "api/v1/health")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// browserCommand membuka url di browser bawaan sistem operasi goos.
func browserCommand(goos, url string) *exec.Cmd {
	switch goos {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		return exec.Command("open", url)
	default:
		return exec.Command("xdg-open", url)
	}
}

// openBrowser membuka url di browser bawaan. Kalau gagal, alamatnya ditulis di jendela konsol.
func (s *Server) openBrowser(url string) {
	cmd := browserCommand(runtime.GOOS, url)
	if err := cmd.Start(); err != nil {
		s.Log.WithError(err).Warn("gagal membuka browser")
		fmt.Printf("Browser tidak terbuka sendiri. Buka %s di browser.\n", url)
		return
	}
	go func() { _ = cmd.Wait() }()
}
