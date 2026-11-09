import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router'
import { ApiError } from '../api/client'
import logo from '../assets/logo-fn-boss.jpg'
import { homeFor, useAuth } from '../auth/AuthContext'
import { STORE } from '../config'

export function LoginPage() {
  const { state, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  if (state.status === 'authenticated') {
    return <Navigate to={from ?? homeFor(state.user.role)} replace />
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      const user = await login(username.trim(), password)
      navigate(from ?? homeFor(user.role), { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Terjadi kesalahan. Coba lagi.')
      setSubmitting(false)
    }
  }

  const inputClass =
    'w-full h-12 px-3 bg-white border-2 border-ink rounded-sm text-base placeholder:text-ink-soft/60'

  return (
    <main className="min-h-dvh bg-kunyit grid place-items-center p-4">
      <section aria-labelledby="judul" className="struk w-full max-w-sm bg-paper px-7 pt-8 pb-14">
        {/* Dipotong mengikuti oval, supaya sudut putih gambarnya tidak terlihat. */}
        <img
          src={logo}
          alt={STORE.name}
          width={720}
          height={434}
          className="mx-auto mb-6 w-full max-w-60 h-auto [clip-path:ellipse(50%_50%_at_50%_50%)]"
        />
        <h1 id="judul" className="text-2xl font-bold">
          Masuk ke kasir
        </h1>
        <p className="mt-1 text-ink-soft">Pakai akun yang dibuat admin toko.</p>

        <form onSubmit={(e) => void handleSubmit(e)} className="mt-6 space-y-4">
          <div>
            <label htmlFor="username" className="block mb-1 text-sm font-semibold">
              Username
            </label>
            <input
              id="username"
              name="username"
              type="text"
              autoComplete="username"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              autoFocus
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className={inputClass}
            />
          </div>

          <div>
            <div className="flex items-baseline justify-between">
              <label htmlFor="password" className="block mb-1 text-sm font-semibold">
                Password
              </label>
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                className="py-2 -my-2 text-sm font-semibold underline underline-offset-2 cursor-pointer"
              >
                {showPassword ? 'Sembunyikan' : 'Tampilkan'}
              </button>
            </div>
            <input
              id="password"
              name="password"
              type={showPassword ? 'text' : 'password'}
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={inputClass}
            />
          </div>

          {error && (
            <p role="alert" className="font-semibold text-merah">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="w-full h-12 bg-ink text-paper text-base font-bold rounded-sm cursor-pointer disabled:opacity-60 disabled:cursor-wait"
          >
            {submitting ? 'Memeriksa…' : 'Masuk'}
          </button>
        </form>
      </section>
    </main>
  )
}
