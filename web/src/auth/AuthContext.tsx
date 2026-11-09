import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { ApiError, login as apiLogin, logout as apiLogout, refreshSession, setSessionLostHandler } from '../api/client'
import type { Role, User } from '../api/types'
import { claimSavedCart, clearSavedCart } from '../lib/cartStorage'

type AuthState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'anonymous' }
  | { status: 'authenticated'; user: User }

interface AuthContextValue {
  state: AuthState
  login: (username: string, password: string) => Promise<User>
  logout: () => Promise<void>
  retry: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function homeFor(role: Role): string {
  return role === 'admin' ? '/laporan' : '/kasir'
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' })

  const restore = useCallback(async () => {
    setState({ status: 'loading' })
    try {
      const session = await refreshSession()
      if (session) claimSavedCart(session.user.id)
      setState(session ? { status: 'authenticated', user: session.user } : { status: 'anonymous' })
    } catch (e) {
      setState({
        status: 'error',
        message: e instanceof ApiError ? e.message : 'Terjadi kesalahan saat memuat sesi.',
      })
    }
  }, [])

  useEffect(() => {
    void restore()
  }, [restore])

  useEffect(() => {
    setSessionLostHandler(() => setState({ status: 'anonymous' }))
    return () => setSessionLostHandler(null)
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({
      state,
      login: async (username, password) => {
        const session = await apiLogin(username, password)
        claimSavedCart(session.user.id)
        setState({ status: 'authenticated', user: session.user })
        return session.user
      },
      // Layar ini selalu keluar dan keranjang dikosongkan, juga kalau server belum terjangkau. Pencabutan sesi di
      // server diulang saat sesi akan dipulihkan (tanda keluar di api/client).
      logout: async () => {
        clearSavedCart()
        setState({ status: 'anonymous' })
        try {
          await apiLogout()
        } catch {
          // offline atau server error: tanda keluar tetap ada, jadi sesi ini tidak dipulihkan lagi
        }
      },
      retry: () => void restore(),
    }),
    [state, restore],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth harus dipakai di dalam AuthProvider')
  return ctx
}
