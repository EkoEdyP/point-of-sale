import { clearPendingKeys } from './requestKey'

// Keranjang kasir disimpan per tab di sessionStorage supaya tidak hilang kalau halaman dimuat ulang di tengah
// transaksi. Keranjang itu milik kasir yang sedang masuk: dikosongkan saat keluar, atau saat kasir lain masuk di tab
// yang sama. Kunci idempotensi yang belum selesai (lib/requestKey.ts) ikut dibuang bersamanya.
export const CART_STORAGE_KEY = 'pos.cart'
const OWNER_KEY = 'pos.cart.owner'

export function clearSavedCart() {
  clearPendingKeys()
  try {
    sessionStorage.removeItem(CART_STORAGE_KEY)
    sessionStorage.removeItem(OWNER_KEY)
  } catch {
    // penyimpanan diblokir: tidak ada keranjang yang tersimpan
  }
}

// Dipanggil setiap sesi dimulai atau dipulihkan. Keranjang tanpa pemilik (disimpan sebelum aturan ini ada) dianggap
// milik user yang sedang masuk.
export function claimSavedCart(userId: string) {
  try {
    const owner = sessionStorage.getItem(OWNER_KEY)
    if (owner !== null && owner !== userId) {
      sessionStorage.removeItem(CART_STORAGE_KEY)
      clearPendingKeys()
    }
    sessionStorage.setItem(OWNER_KEY, userId)
  } catch {
    // penyimpanan diblokir: tidak ada keranjang yang tersimpan
  }
}
