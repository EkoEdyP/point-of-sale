# POS FN Boss

Aplikasi kasir (point of sale) toko FN Boss: penjualan di toko, pre-order, pengiriman, dan laporan. Untuk satu toko
(single tenant). Servernya berjalan di PC toko, lalu aplikasinya dibuka lewat browser di PC itu, atau dari HP dan
tablet di Wi-Fi toko.

- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [How to Run](#how-to-run)
- [How to Backup Database](#how-to-backup-database)
- [Fitur](#fitur)
- [Tes](#tes)

## Tech Stack

| Bagian | Teknologi |
| --- | --- |
| Server (backend) | Go 1.26, Fiber v3 |
| Database | PostgreSQL 15 atau lebih baru, struktur tabel lewat golang-migrate |
| Tampilan (frontend) | React 19, TypeScript 7, Vite 8, Tailwind CSS 4, React Router 8 |
| Login | JWT (golang-jwt) dan refresh token di cookie, password di-hash dengan bcrypt |
| Library Go lain | lib/pq (driver PostgreSQL), validator, viper (membaca `env.sh`), logrus (log ke file) |
| Pengembangan | Make, Air (server dibangun dan dijalankan ulang otomatis setiap file `.go` berubah) |
| Cetak | Dialog cetak browser: struk thermal 57 mm, nota pre-order dan surat jalan 9½ × 5½ inci (Epson LQ-310) |

Struktur folder:

| Folder | Isi |
| --- | --- |
| `main.go`, `delivery/` | server HTTP: route, controller, middleware, dan tampilan web dari `web/dist` |
| `usecase/` | aturan bisnis: penjualan, pre-order, pengiriman, kuota, laporan |
| `repository/postgres/` | query SQL dan riwayat perubahan |
| `model/`, `config/`, `manager/` | struktur data, pengaturan dari `env.sh`, penyusun komponen server |
| `exception/`, `utility/` | error API, token login, dan hash password |
| `db/migrations/` | struktur database |
| `cmd/seed-admin/` | membuat akun admin pertama |
| `web/` | tampilan (React) |

## Prerequisites

Yang perlu di-download dan di-install dulu, sekali saja:

| Program | Versi | Untuk |
| --- | --- | --- |
| Git | terbaru | mengambil kode dari GitHub |
| Go | 1.26 atau lebih baru | server |
| Node.js | LTS (minimal 20.19 atau 22.12) | membangun tampilan web |
| PostgreSQL | 15 atau lebih baru | database |
| golang-migrate | v4 | membuat tabel database |
| Chrome atau Edge | terbaru | membuka aplikasi dan mencetak |
| Air (opsional) | terbaru | `make dev` di Mac |

### Mac

Pasang [Homebrew](https://brew.sh) dulu (sekaligus memasang git dan make), lalu di Terminal:

```bash
brew install go node postgresql@15
brew link --force postgresql@15
brew services start postgresql@15
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc && source ~/.zshrc
go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install github.com/air-verse/air@latest
```

`brew link` membuat `psql`, `createdb`, dan `pg_dump` bisa dipanggil langsung, dan `brew services` menyalakan
PostgreSQL otomatis setiap Mac menyala. Baris `echo` menambahkan folder program dari `go install` ke PATH. Air (baris
terakhir) hanya dibutuhkan untuk `make dev`.

### Windows

1. Download dan install:
   - [Git for Windows](https://git-scm.com/download/win), sekaligus Git Bash. Semua perintah di README ini
     dijalankan di Git Bash.
   - [Go](https://go.dev/dl) (file `.msi`).
   - [Node.js](https://nodejs.org) versi LTS.
   - [PostgreSQL](https://www.postgresql.org/download/windows/) (installer EDB). Catat password user `postgres`
     yang dibuat saat instalasi.
2. Buka Git Bash, lalu jalankan (ganti 17 dengan versi PostgreSQL yang terpasang):

   ```bash
   echo 'export PATH="$PATH:/c/Program Files/PostgreSQL/17/bin"' >> ~/.bashrc && source ~/.bashrc
   go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
   ```

### Lainnya

- Zona waktu PC harus (UTC+07:00) Jakarta, karena tanggal pre-order dicek dengan jam PC.
- Printer: driver printer thermal 57 mm untuk struk, dan Epson LQ-310 untuk nota pre-order dan surat jalan. Untuk
  LQ-310, buat ukuran kertas 241,3 × 139,7 mm (Windows: Print server properties → Forms), lalu jadikan kertas
  bawaannya.

## How to Run

### 1. Persiapan (sekali saja)

Di Terminal (Mac) atau Git Bash (Windows):

```bash
git clone https://github.com/EkoEdyP/point-of-sale.git
cd point-of-sale
cp env.example.sh env.sh
sed -i.bak "s/^export APP_SIGNATURE_KEY=.*/export APP_SIGNATURE_KEY=$(openssl rand -hex 32)/" env.sh && rm env.sh.bak
```

Baris `sed` mengisi kunci login dengan nilai acak. Lalu buka `env.sh` dengan editor teks (Mac: `nano env.sh`,
Windows: `notepad env.sh`) dan sesuaikan:

| Nama | Isi |
| --- | --- |
| `APP_DB_USER` | user PostgreSQL. Mac (Homebrew): nama user Mac. Windows: `postgres` |
| `APP_DB_PASSWORD` | password user itu, huruf dan angka saja. Mac (Homebrew): isi apa saja, tidak dicek |
| `APP_DB_NAME` | nama database, bawaan `pos` |
| `APP_API_HOST` | `localhost`: hanya dari PC ini. `0.0.0.0`: juga dari HP dan tablet di Wi-Fi toko |
| `APP_API_PORT` | port server, bawaan `8080` |

`env.sh` berisi password dan kunci rahasia, jadi tidak ikut ke GitHub (sudah ada di `.gitignore`).

Buat database, tabel, dan akun admin pertama:

```bash
source env.sh
PGPASSWORD="$APP_DB_PASSWORD" createdb -h "$APP_DB_HOST" -p "$APP_DB_PORT" -U "$APP_DB_USER" "$APP_DB_NAME"
PGPASSWORD="$APP_DB_PASSWORD" migrate -path db/migrations \
  -database "postgresql://$APP_DB_USER@$APP_DB_HOST:$APP_DB_PORT/$APP_DB_NAME?sslmode=$APP_SSL_MODE" up
read -rs POS_ADMIN_PASSWORD && export POS_ADMIN_PASSWORD
go run ./cmd/seed-admin --username admin --name "Admin Toko"
unset POS_ADMIN_PASSWORD
```

Setelah baris `read`, ketik password admin (minimal 8 karakter, tidak tampil di layar), lalu Enter. Akun kasir
dibuat oleh admin di menu Pengguna.

### 2. PC kasir (Windows): start.bat

Setiap hari: klik dua kali `start.bat` (bisa dibuatkan shortcut di desktop).

- start.bat membangun tampilan web dan server dari kode di folder ini, menjalankan server, lalu membuka aplikasinya
  di browser. Pertama kali agak lama karena memasang paket web.
- Jendelanya harus tetap terbuka selama toko buka. Menutupnya mematikan server. Kalau hanya browsernya yang
  tertutup, klik start.bat lagi: aplikasinya dibuka lagi tanpa membangun ulang.
- Kalau build gagal, hasil build sebelumnya yang dipakai.

### 3. Pengembangan (Mac/Linux)

Dua terminal, keduanya di folder proyek. Terminal pertama menjalankan server di port 8080, yang dibangun dan
dijalankan ulang otomatis setiap file `.go` berubah:

```bash
make dev
```

Terminal kedua menjalankan tampilan web. `npm install` hanya perlu pertama kali dan setiap `package.json` berubah:

```bash
cd web
npm install
npm run dev
```

Buka http://localhost:5173. Tampilan langsung berubah setiap kode web disimpan.

`make run` menjalankan server tanpa Air. Tanpa Vite: jalankan `npm run build` di `web/`, lalu buka
http://localhost:8080 (ulangi `npm run build` setiap kode web berubah). Cek server:
`curl localhost:8080/api/v1/health`.

### 4. Dari HP atau tablet

1. Isi `APP_API_HOST` dengan `0.0.0.0` di `env.sh`, lalu jalankan ulang server.
2. Windows: saat Windows Firewall bertanya, izinkan pos.exe untuk jaringan Private.
3. Di HP, buka alamat Wi-Fi yang tertulis di jendela start.bat, misalnya `http://192.168.1.8:8080`. Di Mac, alamat
   IP-nya ada di System Settings → Wi-Fi → Details.

### 5. Update ke versi terbaru

1. Backup database dulu (lihat [How to Backup Database](#how-to-backup-database)).
2. Matikan server: tutup jendela start.bat (Windows), atau Ctrl+C di `make dev` (Mac).
3. Jalankan `git pull` di folder proyek.
4. Jalankan lagi: start.bat (Windows) atau `make dev` (Mac).

Kalau pembaruan mengubah struktur database, `git pull` saja tidak cukup: databasenya perlu diperbarui juga, sesuai
catatan pembaruan itu.

## How to Backup Database

Backup memakai `pg_dump`, yang terpasang bersama PostgreSQL. Hasilnya satu file `.dump` berisi semua data: produk,
transaksi, pre-order, pengiriman, akun, dan riwayat perubahan. Server tidak perlu dimatikan.

### Membuat backup

Di Terminal (Mac) atau Git Bash (Windows):

```bash
cd ~/point-of-sale
source env.sh
umask 077
mkdir -p ~/Downloads/pos-backups
FILE=~/Downloads/pos-backups/pos-$(date +%Y%m%d-%H%M).dump
PGPASSWORD="$APP_DB_PASSWORD" pg_dump -h "$APP_DB_HOST" -p "$APP_DB_PORT" -U "$APP_DB_USER" \
  -Fc -f "$FILE" "$APP_DB_NAME"
pg_restore --list "$FILE" > /dev/null && echo "Backup selesai: $FILE"
```

Ganti `~/point-of-sale` dengan folder proyek. `umask 077` membuat file backup hanya bisa dibaca akun ini, dan baris
terakhir memeriksa bahwa file backup bisa dibaca. Hasilnya misalnya `~/Downloads/pos-backups/pos-20261109-2130.dump`
(di Windows: `C:\Users\<nama>\Downloads\pos-backups`).

- Backup setiap hari setelah toko tutup, dan sebelum update aplikasi.
- Simpan salinannya di tempat lain juga, misalnya flashdisk atau Google Drive. Backup yang hanya ada di PC toko ikut
  hilang kalau PC-nya rusak.
- File backup berisi data pelanggan (nama, nomor HP, alamat) dan hash password akun. Jangan dibagikan, dan jangan
  disimpan di folder proyek, karena repo ini publik. Untuk berjaga-jaga, `*.dump` sudah ada di `.gitignore`.

### Memulihkan dari backup (restore)

Matikan server POS dulu. Database yang sekarang tidak dihapus: namanya diganti menjadi `pos_lama_<tanggal jam>`,
lalu database baru dibuat dari file backup.

```bash
cd ~/point-of-sale
source env.sh
FILE=~/Downloads/pos-backups/pos-20261109-2130.dump
LAMA="${APP_DB_NAME}_lama_$(date +%Y%m%d%H%M)"
PGPASSWORD="$APP_DB_PASSWORD" psql -h "$APP_DB_HOST" -p "$APP_DB_PORT" -U "$APP_DB_USER" -d postgres \
  -c "ALTER DATABASE \"$APP_DB_NAME\" RENAME TO \"$LAMA\"" &&
PGPASSWORD="$APP_DB_PASSWORD" createdb -h "$APP_DB_HOST" -p "$APP_DB_PORT" -U "$APP_DB_USER" "$APP_DB_NAME" &&
PGPASSWORD="$APP_DB_PASSWORD" pg_restore -h "$APP_DB_HOST" -p "$APP_DB_PORT" -U "$APP_DB_USER" \
  --no-owner --no-privileges -d "$APP_DB_NAME" "$FILE"
```

Ganti nama file di baris `FILE=` dengan file yang mau dipulihkan, lalu jalankan server lagi setelah selesai. Tiga
perintah terakhir disambung `&&`, jadi kalau satu gagal, sisanya tidak dijalankan. Kalau muncul "is being accessed
by other users", berarti server masih berjalan: matikan dulu, lalu ulangi. Database lama tetap ada sampai dihapus
sendiri dengan `dropdb`.

Pulihkan ke PostgreSQL versi yang sama atau lebih baru dari versi tempat backup dibuat.

### Pindah ke PC lain

Di PC baru, pasang semua [Prerequisites](#prerequisites), lalu ikuti [Persiapan](#1-persiapan-sekali-saja) sampai
`env.sh` selesai diisi. Sebagai ganti `migrate` dan `seed-admin`, salin file backup ke folder `Downloads/pos-backups`
di PC baru, lalu jalankan di folder proyek:

```bash
source env.sh
PGPASSWORD="$APP_DB_PASSWORD" createdb -h "$APP_DB_HOST" -p "$APP_DB_PORT" -U "$APP_DB_USER" "$APP_DB_NAME" &&
PGPASSWORD="$APP_DB_PASSWORD" pg_restore -h "$APP_DB_HOST" -p "$APP_DB_PORT" -U "$APP_DB_USER" \
  --no-owner --no-privileges -d "$APP_DB_NAME" ~/Downloads/pos-backups/pos-20261109-2130.dump
```

Semua akun ikut pindah, lengkap dengan password-nya.

## Fitur

Label `ADMIN` untuk pemilik atau pengelola toko, `KASIR` untuk akun kasir.

| Menu | ADMIN | KASIR |
| --- | :---: | :---: |
| Kasir | ✓ | ✓ |
| Pre-order | ✓ | ✓ |
| Pengiriman | ✓ | ✓ |
| Riwayat | ✓ semua transaksi | ✓ transaksi sendiri |
| Produk | ✓ | – |
| Laporan | ✓ | – |
| Pengguna | ✓ | – |

### Akun dan tampilan

- `ADMIN` `KASIR` Login dengan username dan password. Sesi tersimpan di browser, jadi tidak perlu login setiap
  membuka aplikasi.
- `ADMIN` `KASIR` Setelah 5 kali salah password, login akun itu dikunci sementara: 30 detik, lalu dua kali lebih lama
  setiap gagal lagi, paling lama 15 menit.
- `ADMIN` `KASIR` Angka pengingat di menu Pre-order dan Pengiriman: jumlah pesanan yang terlambat, jatuh tempo hari
  ini, besok, dan lusa.
- `ADMIN` `KASIR` Tampilan menyesuaikan HP, tablet, dan PC.
- `ADMIN` Halaman pertama setelah login: Laporan.
- `KASIR` Halaman pertama setelah login: Kasir.

### Kasir

- `ADMIN` `KASIR` Daftar produk toko dengan harga dan stok. Cari nama atau SKU, lalu Enter langsung menambahkan
  hasil pertama ke keranjang.
- `ADMIN` `KASIR` Keranjang: ubah jumlah, hapus barang, atau kosongkan semuanya.
- `ADMIN` `KASIR` Harga grosir otomatis kalau jumlah satu produk mencapai batasnya.
- `ADMIN` `KASIR` Bayar dengan Tunai (uang diterima dan kembalian), QRIS, E-wallet, atau Kartu. Stok berkurang
  otomatis.
- `ADMIN` `KASIR` Struk thermal 57 mm dengan logo toko.
- `ADMIN` `KASIR` Penjualan tidak tercatat dua kali kalau koneksi putus saat membayar. Kalau harga berubah sebelum
  dibayar, kasir diminta mengecek total yang baru.

### Pre-order

- `ADMIN` `KASIR` Buat pre-order: pemesan dan nomor HP, diambil di toko atau diantar (dengan alamat), tanggal dan
  jam (hari H), barang, ongkir, biaya lain (misalnya kresek), DP beserta metode bayarnya, dan catatan. Harga barang
  dikunci saat pre-order dibuat.
- `ADMIN` `KASIR` Nota pre-order untuk Epson LQ-310 (9½ × 5½ inci) langsung terbuka untuk dicetak, dan bisa dicetak
  lagi dari detail pre-order.
- `ADMIN` `KASIR` Daftar pre-order dengan filter status (Berjalan, Selesai, Dibatalkan) dan hari H (Hari ini,
  Besok, atau tanggal tertentu). Warna menandai yang terlambat, hari ini, besok, dan lusa.
- `ADMIN` `KASIR` Kuota harian: total pcs pre-order per hari H dibatasi, dan sisa kuotanya tampil saat memilih
  tanggal.
- `ADMIN` Ubah kuota harian (awalnya 5.000 pcs).
- `ADMIN` `KASIR` Tambah DP, Tandai siap, lalu Serahkan dan selesaikan: sisa tagihan dibayar dan struk dicetak.
  Untuk pre-order yang diantar, pengirimannya ikut selesai dan nama penerima wajib diisi.
- `ADMIN` `KASIR` Ubah data pelanggan, jadwal, ongkir, dan biaya lain selama pre-order belum selesai. Perubahan
  data pelanggan dan jadwal ikut ke pengirimannya.
- `ADMIN` `KASIR` Batalkan pre-order dengan alasan. Kasir hanya bisa membatalkan pre-order buatannya sendiri.
- `ADMIN` `KASIR` Kebutuhan produksi: per hari H dan per produk, berapa yang dipesan, yang sudah siap, dan
  kekurangannya.
- `ADMIN` Catat produksi: menambah stok siap produk pre-order setiap ada barang yang selesai dibuat.

### Pengiriman

- `ADMIN` `KASIR` Daftar pengiriman dari pre-order yang diantar, dengan status Menunggu, Dalam perjalanan,
  Terkirim, Gagal, dan Dibatalkan.
- `ADMIN` `KASIR` Berangkat (dengan nama kurir), Terkirim, Tandai gagal, dan Jadwalkan ulang. Saat Terkirim, nama
  penerima wajib diisi, sisa tagihan dibayar ke kurir, dan pre-order-nya ikut selesai.
- `ADMIN` `KASIR` Ubah pengiriman: data pemesan, alamat, jadwal, dan catatan ikut berubah di pre-order-nya.
- `ADMIN` `KASIR` Surat jalan seukuran nota (9½ × 5½ inci), lengkap dengan harga dan tagihan yang dibawa kurir.
- `ADMIN` `KASIR` Batalkan pengiriman dengan alasan. Kasir hanya bisa membatalkan pengiriman buatannya sendiri.

### Riwayat

- `ADMIN` Semua transaksi toko, lengkap dengan nama kasirnya.
- `KASIR` Hanya transaksi yang dibuatnya sendiri.
- `ADMIN` `KASIR` Filter tanggal (Hari ini, Kemarin, 7 hari terakhir, atau tanggal bebas) dan status (Selesai,
  Dibatalkan). Label Toko atau PO menandai asal transaksi.
- `ADMIN` `KASIR` Detail transaksi dan Cetak ulang struk. Untuk transaksi dari pre-order ada juga Cetak ulang nota.
- `ADMIN` `KASIR` Batalkan transaksi dengan alasan, dan stoknya kembali. Kasir hanya bisa membatalkan transaksinya
  sendiri. Transaksi dari pre-order tidak bisa dibatalkan. Siapa yang membatalkan, kapan, dan alasannya ikut
  tercatat.

### Produk (hanya ADMIN)

- `ADMIN` Produk toko dan produk pre-order di tab terpisah (Toko dan PO).
- `ADMIN` Tambah, ubah, dan hapus produk: nama, SKU, harga jual, harga grosir beserta mulai dari berapa pcs, stok,
  dan tampil di kasir atau tidak.
- `ADMIN` Catat produksi untuk produk pre-order.

### Laporan (hanya ADMIN)

- `ADMIN` Laporan Toko, PO, atau Semua, untuk Hari ini, Kemarin, 7 hari terakhir, Bulan ini, atau tanggal bebas.
- `ADMIN` Omzet, jumlah transaksi, barang terjual, rata-rata per transaksi, uang masuk per metode bayar (DP dihitung
  pada hari DP diterima), pembatalan, dan omzet per hari.

### Pengguna (hanya ADMIN)

- `ADMIN` Tambah pengguna: nama, username, password, dan peran (admin atau kasir). Peran tidak bisa diubah setelah
  akun dibuat.
- `ADMIN` Ubah nama, reset password, serta nonaktifkan dan aktifkan lagi akun. Kasir yang lupa password meminta
  admin me-reset-nya.

### Riwayat perubahan (hanya ADMIN)

- `ADMIN` Setiap perubahan pre-order (termasuk setiap DP), pengiriman, produk, pengguna, dan kuota tercatat: siapa,
  kapan, nilai lama dan nilai baru, serta alasan pembatalan. Dibuka dari Lainnya → Riwayat perubahan di detail
  pre-order dan pengiriman, atau dari tombol Riwayat di form produk, dialog kuota, dan daftar pengguna.

## Tes

- `make test`: tes tanpa database. Tes yang butuh database dilewati (SKIP).
- `make test-db`: tes lengkap. Database percobaan `pos_check` dibuat ulang dari `db/migrations`, lalu semua tes
  dijalankan di sana, termasuk penjualan, pembatalan, pre-order, pengiriman, kuota, laporan, dan aturan database.
  Database toko (`APP_DB_NAME` di `env.sh`) tidak disentuh. Tes database hanya berjalan kalau `APP_DB_NAME=pos_check`.
