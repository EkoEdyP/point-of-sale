import { Navigate, Outlet, useLocation } from 'react-router'
import type { Role } from '../api/types'
import { homeFor, useAuth } from './AuthContext'

function FullScreenMessage({ children }: { children: React.ReactNode }) {
  return (
    <main className="min-h-dvh grid place-items-center p-6 text-center">
      <div className="max-w-sm space-y-4">{children}</div>
    </main>
  )
}

// Semua halaman di dalamnya butuh login.
export function RequireAuth() {
  const { state, retry } = useAuth()
  const location = useLocation()

  if (state.status === 'loading') {
    return (
      <FullScreenMessage>
        <p className="text-ink-soft">Memuat sesi…</p>
      </FullScreenMessage>
    )
  }

  if (state.status === 'error') {
    return (
      <FullScreenMessage>
        <p className="font-semibold">{state.message}</p>
        <button
          type="button"
          onClick={retry}
          className="h-12 px-6 bg-ink text-paper font-bold rounded-sm cursor-pointer"
        >
          Coba lagi
        </button>
      </FullScreenMessage>
    )
  }

  if (state.status === 'anonymous') {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }

  return <Outlet />
}

// Pembatasan per peran. Dipasang di dalam RequireAuth.
export function RequireRole({ roles }: { roles: Role[] }) {
  const { state } = useAuth()
  if (state.status !== 'authenticated') return null
  if (!roles.includes(state.user.role)) {
    return <Navigate to={homeFor(state.user.role)} replace />
  }
  return <Outlet />
}
