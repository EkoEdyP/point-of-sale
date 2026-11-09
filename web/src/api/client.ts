import type { AuthResponse } from './types'

export class ApiError extends Error {
  status: number
  // Kode untuk kasus yang ditangani client, misalnya "price_changed" (muat ulang harga) atau "duplicate_request"
  // (kunci idempotensi sudah dipakai penyimpanan lain).
  code?: string

  constructor(status: number, message: string, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

// Access token hanya disimpan di memori (bukan localStorage), jadi hilang saat tab ditutup.
// Saat halaman dimuat ulang, sesi dipulihkan lewat cookie refresh_token (HttpOnly).
let accessToken: string | null = null

// Tanda keluar yang belum sampai ke server (misalnya saat offline). Selama tanda ini ada, sesi tidak dipulihkan dari
// cookie, supaya orang berikutnya tidak masuk sebagai kasir sebelumnya hanya dengan memuat ulang halaman.
const LOGOUT_PENDING_KEY = 'pos.logoutPending'

function setLogoutPending(pending: boolean) {
  try {
    if (pending) localStorage.setItem(LOGOUT_PENDING_KEY, '1')
    else localStorage.removeItem(LOGOUT_PENDING_KEY)
  } catch {
    // penyimpanan diblokir: keluar tetap jalan, hanya tanpa pengulangan saat offline
  }
}

function logoutPending(): boolean {
  try {
    return localStorage.getItem(LOGOUT_PENDING_KEY) !== null
  } catch {
    return false
  }
}

let onSessionLost: (() => void) | null = null

export function setSessionLostHandler(handler: (() => void) | null) {
  onSessionLost = handler
}

async function rawFetch(path: string, init: RequestInit = {}, withAuth = true): Promise<Response> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (withAuth && accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`)
  }
  try {
    return await fetch(path, { ...init, headers, credentials: 'same-origin' })
  } catch {
    throw new ApiError(0, 'Tidak dapat terhubung ke server. Periksa jaringan toko.')
  }
}

async function toApiError(res: Response): Promise<ApiError> {
  let message = 'Terjadi kesalahan pada server.'
  let code: string | undefined
  try {
    const body = (await res.json()) as { error?: string; code?: string }
    if (body.error) message = body.error
    code = body.code
  } catch {
    // body bukan JSON, pakai pesan umum
  }
  return new ApiError(res.status, message, code)
}

// Refresh token diputar (rotation) di server, jadi token yang sama tidak bisa dipakai dua kali. Di tab ini semua
// pemanggil berbagi satu request yang sedang berjalan. Kalau tab lain baru saja memutar token yang sama, server
// membalas 409; cookie baru dari tab itu segera tersimpan di browser, jadi request diulang (paling banyak dua kali).
let refreshing: Promise<AuthResponse | null> | null = null

const ROTATED_RETRY_MS = [500, 1000]

function postRefresh(): Promise<Response> {
  return rawFetch('/api/v1/auth/refresh', { method: 'POST' }, false)
}

// Mengembalikan null kalau sesi tidak ada atau sudah tidak berlaku, dan melempar ApiError kalau server tidak terjangkau.
export function refreshSession(): Promise<AuthResponse | null> {
  if (!refreshing) {
    refreshing = (async () => {
      try {
        if (logoutPending()) {
          accessToken = null
          try {
            await finishLogout()
          } catch {
            // server belum terjangkau: tandanya tetap ada dan diulang saat sesi dipulihkan berikutnya
          }
          return null
        }
        let res = await postRefresh()
        for (const wait of ROTATED_RETRY_MS) {
          if (res.status !== 409) break
          await new Promise((resolve) => setTimeout(resolve, wait))
          res = await postRefresh()
        }
        if (res.status === 401 || res.status === 409) {
          accessToken = null
          return null
        }
        if (!res.ok) throw await toApiError(res)
        const data = (await res.json()) as AuthResponse
        accessToken = data.access_token
        return data
      } finally {
        refreshing = null
      }
    })()
  }
  return refreshing
}

export async function login(username: string, password: string): Promise<AuthResponse> {
  const res = await rawFetch(
    '/api/v1/auth/login',
    { method: 'POST', body: JSON.stringify({ username, password }) },
    false,
  )
  if (!res.ok) throw await toApiError(res)
  const data = (await res.json()) as AuthResponse
  accessToken = data.access_token
  // Cookie sesi baru menggantikan yang lama, jadi keluar yang tertunda tidak perlu diulang.
  setLogoutPending(false)
  return data
}

// Access token dibuang dan tanda keluar dipasang lebih dulu, baru server mencabut refresh token dan menghapus
// cookie-nya. Melempar ApiError kalau server tidak terjangkau; tandanya tetap ada dan diulang saat sesi akan
// dipulihkan.
export async function logout(): Promise<void> {
  accessToken = null
  setLogoutPending(true)
  await finishLogout()
}

async function finishLogout(): Promise<void> {
  const res = await rawFetch('/api/v1/auth/logout', { method: 'POST' }, false)
  if (!res.ok) throw await toApiError(res)
  setLogoutPending(false)
}

// Request ke API yang butuh login. Kalau access token sudah kedaluwarsa (401),
// sesi diperbarui sekali lalu request diulang.
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  let res = await rawFetch(path, init)

  if (res.status === 401 && accessToken) {
    const session = await refreshSession()
    if (!session) {
      onSessionLost?.()
      throw new ApiError(401, 'Sesi berakhir. Silakan masuk lagi.')
    }
    res = await rawFetch(path, init)
  }

  if (!res.ok) throw await toApiError(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}
