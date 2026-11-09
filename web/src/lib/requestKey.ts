import { ApiError } from '../api/client'

// Kunci idempotensi (header Idempotency-Key) untuk penyimpanan yang tidak boleh tercatat dua kali: penjualan,
// pre-order, DP, pelunasan pre-order, dan hasil produksi. Satu hal yang mau disimpan (scope, misalnya
// "dp:<id pre-order>") memakai kunci yang sama sampai berhasil, jadi kalau jawaban server hilang di jaringan setelah
// datanya tersimpan, mengirim ulang tidak menyimpannya kedua kalinya. Kunci disimpan per tab di sessionStorage,
// supaya tetap sama walaupun halaman dimuat ulang.
const STORAGE_KEY = 'pos.requestKeys'

let memory: Record<string, string> = {}

function load(): Record<string, string> {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    const parsed: unknown = raw ? JSON.parse(raw) : {}
    return typeof parsed === 'object' && parsed !== null ? (parsed as Record<string, string>) : {}
  } catch {
    return memory
  }
}

function save(keys: Record<string, string>) {
  memory = keys
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(keys))
  } catch {
    // penyimpanan diblokir: kunci tetap ada di memori selama tab terbuka
  }
}

// crypto.randomUUID hanya ada di secure context (HTTPS atau localhost), sedangkan HP kasir membuka aplikasi lewat
// HTTP di jaringan toko, jadi UUID v4 dibuat dari crypto.getRandomValues.
export function newRequestKey(): string {
  const b = crypto.getRandomValues(new Uint8Array(16))
  b[6] = (b[6] & 0x0f) | 0x40
  b[8] = (b[8] & 0x3f) | 0x80
  const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`
}

// Kunci untuk scope ini: yang sudah ada dipakai lagi, atau dibuat baru.
export function pendingKey(scope: string): string {
  const keys = load()
  if (!keys[scope]) {
    keys[scope] = newRequestKey()
    save(keys)
  }
  return keys[scope]
}

// Dipanggil setelah penyimpanan berhasil, atau setelah server menjawab bahwa kuncinya sudah dipakai untuk isi yang
// lain: percobaan berikutnya dianggap penyimpanan baru.
export function settleKey(scope: string) {
  const keys = load()
  if (scope in keys) {
    delete keys[scope]
    save(keys)
  }
}

// Saat keluar atau saat pengguna lain masuk di tab ini.
export function clearPendingKeys() {
  memory = {}
  try {
    sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    // penyimpanan diblokir: hanya memori yang dikosongkan
  }
}

export function idempotencyHeader(key: string | undefined): HeadersInit | undefined {
  return key ? { 'Idempotency-Key': key } : undefined
}

// Menjalankan penyimpanan dengan kunci scope. Kuncinya dilepas setelah berhasil, atau setelah server menjawab bahwa
// kunci itu sudah dipakai penyimpanan lain yang tersimpan (pesannya menyebut apa yang sudah tersimpan). Gagal karena
// sebab lain, misalnya jaringan putus, kuncinya tetap untuk percobaan berikutnya.
export async function withRequestKey<T>(scope: string, save: (key: string) => Promise<T>): Promise<T> {
  try {
    const result = await save(pendingKey(scope))
    settleKey(scope)
    return result
  } catch (err) {
    if (err instanceof ApiError && err.code === 'duplicate_request') settleKey(scope)
    throw err
  }
}
