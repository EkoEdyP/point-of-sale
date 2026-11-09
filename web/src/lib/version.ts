import { ApiError } from '../api/client'

// Data pre-order dan pengiriman membawa `version`, yang naik setiap kali datanya berubah. Dialog ubah mengirim versi
// saat dibuka, dan server menolak (409, code "stale") kalau datanya sudah diubah orang lain sejak itu, supaya isian
// lama di dialog tidak menimpa perubahan tersebut.
export const isStale = (e: unknown) => e instanceof ApiError && e.code === 'stale'

export const staleMessage = (what: string) =>
  `${what} ini sudah diubah orang lain sejak dialog ini dibuka. Muat ulang untuk melihat data terbaru, ` +
  'lalu ulangi perubahannya.'
