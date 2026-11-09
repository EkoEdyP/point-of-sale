# pos

Point of sale (single tenant), server lokal di LAN toko.

## Menjalankan

1. `cp env.example.sh env.sh`, lalu isi nilainya (`env.sh` sudah ada di `.gitignore`)
2. `source env.sh`
3. Buat database kosong, lalu `make migrate-up` (butuh golang-migrate)
4. `make seed-admin USERNAME=admin NAME="Admin Toko"`
5. `make run`, atau `make dev` supaya server dibangun dan dijalankan ulang otomatis setiap file `.go` berubah
   (butuh [Air](https://github.com/air-verse/air): `go install github.com/air-verse/air@latest`)
6. Cek: `curl localhost:8080/api/v1/health`

## Tes

- `make test`: tes tanpa database. Tes yang butuh database dilewati (SKIP).
- `make test-db`: tes lengkap. Database percobaan `pos_check` dibuat ulang dari `db/migrations`, lalu semua tes
  dijalankan di sana, termasuk penjualan, pembatalan, pre-order, pengiriman, kuota, laporan, dan aturan database.
  Database toko (`APP_DB_NAME` di `env.sh`) tidak disentuh. Tes database hanya berjalan kalau `APP_DB_NAME=pos_check`.
