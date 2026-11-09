@echo off
rem Menjalankan POS di PC kasir Windows dengan sekali klik: membangun tampilan web dan server dari kode di folder
rem ini, menjalankan server, lalu membuka aplikasinya di browser. Server berjalan selama jendela ini terbuka.
rem Pengaturan dibaca pos.exe sendiri dari env.sh di folder ini (salinan env.example.sh).
setlocal
title POS FN Boss
cd /d "%~dp0"

if not exist env.sh (
  echo env.sh belum ada. Salin env.example.sh menjadi env.sh, lalu isi nilainya.
  goto gagal
)

rem Kalau server masih berjalan, misalnya hanya browsernya yang ditutup, tidak ada yang dibangun ulang: pos.exe yang
rem sedang berjalan tidak bisa ditimpa. pos.exe kedua hanya membuka browser ke server itu, lalu selesai.
tasklist /nh /fi "imagename eq pos.exe" 2>nul | find /i "pos.exe" >nul
if not errorlevel 1 (
  pos.exe --buka
  if errorlevel 1 goto gagal
  exit /b 0
)

where go >nul 2>nul || (
  echo Go belum terpasang. Pasang dari https://go.dev/dl lalu jalankan lagi.
  goto gagal
)
where npm >nul 2>nul || (
  echo Node.js belum terpasang. Pasang versi LTS dari https://nodejs.org lalu jalankan lagi.
  goto gagal
)

echo [1/3] Membangun tampilan web...
pushd web
rem Paket web dipasang ulang hanya kalau package-lock.json berubah sejak pemasangan terakhir.
fc /b package-lock.json node_modules\.pos-lock >nul 2>nul
if errorlevel 1 (
  call npm ci --no-audit --no-fund
  if errorlevel 1 goto gagal_web
  copy /y package-lock.json node_modules\.pos-lock >nul
)
rem Kalau build gagal, misalnya karena kode sedang diperbarui, hasil build sebelumnya tetap dipakai.
call npm run build
if errorlevel 1 (
  if not exist dist\index.html goto gagal_web
  echo Tampilan web gagal dibangun, memakai hasil sebelumnya.
)
popd

echo [2/3] Membangun server...
rem Server dibangun menjadi pos.exe di folder ini, bukan dijalankan dengan go run: izin Windows Firewall terikat
rem pada lokasi program, jadi izin untuk HP di Wi-Fi toko cukup diberikan sekali.
go build -o pos.exe .
if errorlevel 1 (
  if not exist pos.exe goto gagal
  echo Server gagal dibangun, memakai hasil sebelumnya.
)

echo [3/3] Menjalankan server...
pos.exe --buka
if errorlevel 1 goto gagal
exit /b 0

:gagal_web
popd
:gagal
echo.
echo Gagal. Baca pesan di atas, lalu tekan tombol apa saja untuk menutup jendela ini.
pause >nul
exit /b 1
