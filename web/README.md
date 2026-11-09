# pos-web

Frontend kasir (React + Vite + Tailwind). Berbicara dengan API Go lewat `/api`.

## Development

1. Jalankan server Go di port 8080 (`make run` di folder `pos`).
2. Di folder ini:

```bash
npm install
npm run dev
```

3. Buka http://localhost:5173. Vite meneruskan `/api` ke `localhost:8080`, jadi cookie login ikut terkirim.

## Build

```bash
npm run build   # hasilnya di dist/
```
