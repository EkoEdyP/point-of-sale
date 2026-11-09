#!/bin/bash
# Jangan menulis komentar di baris yang sama dengan nilai (export X=nilai # komentar).
# Makefile membaca file ini dengan "include", dan spasi sebelum "#" ikut menjadi bagian nilai.
# Server juga membaca env.sh sendiri dari folder tempat ia dijalankan (misalnya lewat start.bat di Windows), jadi
# tidak perlu "source env.sh" dulu. Variabel yang sudah diset di lingkungan tetap dipakai lebih dulu.

# Database Configuration
export APP_DB_DRIVER=postgres
export APP_DB_HOST=localhost
export APP_DB_PORT=5432
export APP_DB_USER=eep
# Wajib diisi, tapi diabaikan kalau Postgres lokal memakai trust auth
export APP_DB_PASSWORD=changeme
export APP_DB_NAME=pos
export APP_SSL_MODE=disable
export APP_TIME_ZONE=Asia/Jakarta

# Api Configuration
# Ganti ke 0.0.0.0 supaya bisa diakses dari LAN toko (HP dan tablet membuka http://<IP PC>:8080)
export APP_API_HOST=localhost
export APP_API_PORT=8080

# File Log Configuration
export APP_FILE_PATH=logger.log

# Token Configuration
export APP_ISSUER=POS
# Wajib dibuat sendiri, jangan memakai contoh (minimal 32 karakter): openssl rand -hex 32
export APP_SIGNATURE_KEY=
export APP_EXPIRES_IN_MINUTES=15
export APP_REFRESH_EXPIRES_IN_DAYS=7

# Opsional: hapus tanda # untuk mengubah nilai bawaan
# Batas koneksi database sekaligus (1-100); request lain mengantre
# export APP_DB_MAX_CONNS=20
# Query yang lebih lama dari ini dibatalkan; kasir menerima "server sedang sibuk, coba lagi"
# export APP_DB_STATEMENT_TIMEOUT=30s
# Batas menunggu data yang sedang dipakai transaksi lain (mis. stok produk yang sama)
# export APP_DB_LOCK_TIMEOUT=10s
# Level log: debug (semua request, termasuk GET), info, warn, error
# export APP_LOG_LEVEL=info
# Folder hasil "npm run build" yang ikut disajikan server (bawaan web/dist). Selama folder itu belum ada, buka
# tampilan web lewat "npm run dev".
# export APP_WEB_DIR=web/dist
