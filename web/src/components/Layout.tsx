import { useEffect, useId, useState, type ReactNode } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router'
import type { Role } from '../api/types'
import logoIcon from '../assets/logo-fn-boss-ikon.png'
import { useAuth } from '../auth/AuthContext'
import { STORE } from '../config'
import { useDeliverySummary } from '../hooks/useDeliverySummary'
import { usePreorderSummary } from '../hooks/usePreorderSummary'
import { TONE_CLASS, type DueTone } from '../lib/preorder'
import { ROLE_LABEL } from '../lib/user'

// Lingkaran logo FN Boss di header.
function HeaderLogo({ className }: { className: string }) {
  return (
    <img
      src={logoIcon}
      alt={STORE.name}
      width={128}
      height={128}
      className={`shrink-0 rounded-full bg-white ${className}`}
    />
  )
}

interface NavItem {
  to: string
  label: string
  roles: Role[]
}

const NAV: NavItem[] = [
  { to: '/kasir', label: 'Kasir', roles: ['admin', 'cashier'] },
  { to: '/preorder', label: 'Pre-order', roles: ['admin', 'cashier'] },
  { to: '/pengiriman', label: 'Pengiriman', roles: ['admin', 'cashier'] },
  { to: '/riwayat', label: 'Riwayat', roles: ['admin', 'cashier'] },
  { to: '/produk', label: 'Produk', roles: ['admin'] },
  { to: '/laporan', label: 'Laporan', roles: ['admin'] },
  { to: '/pengguna', label: 'Pengguna', roles: ['admin'] },
]


// Inisial untuk lingkaran akun: huruf pertama dari maksimal dua kata ("Admin Toko" jadi "AT").
// Array.from supaya karakter di luar BMP (emoji) tidak terpotong setengah.
function initials(name: string): string {
  const letters = name
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => Array.from(w)[0].toUpperCase())
    .join('')
  return letters || '?'
}

function Icon({ children }: { children: ReactNode }) {
  return (
    <svg
      aria-hidden="true"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.25"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {children}
    </svg>
  )
}

const LogoutIcon = () => (
  <Icon>
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
    <polyline points="16 17 21 12 16 7" />
    <line x1="21" y1="12" x2="9" y2="12" />
  </Icon>
)

const MenuIcon = () => (
  <Icon>
    <line x1="4" y1="7" x2="20" y2="7" />
    <line x1="4" y1="12" x2="20" y2="12" />
    <line x1="4" y1="17" x2="20" y2="17" />
  </Icon>
)

const CloseIcon = () => (
  <Icon>
    <line x1="6" y1="6" x2="18" y2="18" />
    <line x1="18" y1="6" x2="6" y2="18" />
  </Icon>
)

function CountBadge({ count }: { count: number }) {
  return (
    <span
      aria-label={`${count} mendekati jatuh tempo`}
      className="min-w-5 h-5 px-1 grid place-items-center rounded-full bg-merah text-white text-xs font-bold"
    >
      {count}
    </span>
  )
}

// Ringkasan jatuh tempo untuk strip peringatan. Hanya pre-order yang punya hitungan lusa,
// karena peringatan pre-order dimulai H-2, sedangkan pengiriman H-1.
interface Summary {
  overdue: number
  today: number
  tomorrow: number
  day_after_tomorrow?: number
}

const urgentCount = (s: Summary | null) =>
  s ? s.overdue + s.today + s.tomorrow + (s.day_after_tomorrow ?? 0) : 0

// Tiap jumlah diberi warnanya sendiri, sama dengan label di daftar:
// merah terlambat/hari ini, kuning besok, hijau lusa.
function summaryParts(s: Summary): { text: string; tone: DueTone }[] {
  const dayAfterTomorrow = s.day_after_tomorrow ?? 0
  const parts: { text: string; tone: DueTone }[] = []
  if (s.overdue > 0) parts.push({ text: `${s.overdue} terlambat`, tone: 'late' })
  if (s.today > 0) parts.push({ text: `${s.today} hari ini`, tone: 'today' })
  if (s.tomorrow > 0) parts.push({ text: `${s.tomorrow} besok`, tone: 'soon' })
  if (dayAfterTomorrow > 0) parts.push({ text: `${dayAfterTomorrow} lusa`, tone: 'upcoming' })
  return parts
}

function SummaryLink({ to, label, summary }: { to: string; label: string; summary: Summary }) {
  return (
    <Link to={to} className="inline-flex flex-wrap items-center gap-1.5">
      <span className="underline underline-offset-2">{label}:</span>
      {summaryParts(summary).map((part) => (
        <span key={part.tone} className={`px-2 py-0.5 rounded-sm text-sm ${TONE_CLASS[part.tone]}`}>
          {part.text}
        </span>
      ))}
    </Link>
  )
}

export function Layout() {
  const { state, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const preorders = usePreorderSummary()
  const deliveries = useDeliverySummary()
  const menuId = useId()
  // Di bawah 1024px menu dilipat ke balik tombol Menu, karena tujuh tautan tidak muat sebaris.
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    if (!menuOpen) return
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setMenuOpen(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [menuOpen])

  if (state.status !== 'authenticated') return null
  const { user } = state

  async function handleLogout() {
    await logout()
    navigate('/login', { replace: true })
  }

  const badge: Record<string, number> = {
    '/preorder': urgentCount(preorders),
    '/pengiriman': urgentCount(deliveries),
  }
  const items = NAV.filter((item) => item.roles.includes(user.role))
  const current = items.find((item) => location.pathname.startsWith(item.to))
  const totalBadge = items.reduce((sum, item) => sum + (badge[item.to] ?? 0), 0)

  return (
    <div className="min-h-dvh flex flex-col">
      <header className="relative z-40 bg-ink text-paper">
        <div className="mx-auto max-w-6xl px-4 flex items-center gap-3 lg:gap-4">
          <div className="lg:hidden flex-1 min-w-0 flex items-center gap-2 py-2">
            <button
              type="button"
              aria-expanded={menuOpen}
              aria-controls={menuId}
              onClick={() => setMenuOpen((open) => !open)}
              className="shrink-0 h-11 pl-2.5 pr-3 inline-flex items-center gap-2 rounded-sm font-semibold hover:bg-white/10 focus-visible:outline-paper cursor-pointer"
            >
              {menuOpen ? <CloseIcon /> : <MenuIcon />}
              {/* Di HP cukup ikon, supaya judul halaman tidak terpotong. */}
              <span className="max-sm:sr-only">Menu</span>
              {!menuOpen && totalBadge > 0 && <CountBadge count={totalBadge} />}
            </button>
            {/* Di HP terkecil (< 360 px) disembunyikan dan di HP lain dibuat 32 px,
                supaya judul halaman terpanjang (Pengiriman) tidak terpotong walau ada angka di tombol Menu. */}
            <HeaderLogo className="max-xs:hidden size-8 sm:size-10" />
            {current && <span className="min-w-0 truncate text-lg font-bold">{current.label}</span>}
          </div>

          <HeaderLogo className="hidden lg:block size-10" />
          <nav aria-label="Menu utama" className="hidden lg:flex flex-1 gap-1 overflow-x-auto py-2">
            {items.map((item) => {
              const count = badge[item.to] ?? 0
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  // px-3 sampai 1280 px: memberi tempat untuk logo, supaya menu admin (7 item + angka) muat di 1024 px.
                  className={({ isActive }) =>
                    `h-11 px-3 xl:px-4 inline-flex items-center gap-2 rounded-sm font-semibold whitespace-nowrap ${
                      isActive ? 'bg-kunyit text-ink' : 'text-paper hover:bg-white/10'
                    }`
                  }
                >
                  {item.label}
                  {count > 0 && <CountBadge count={count} />}
                </NavLink>
              )
            })}
          </nav>

          <div
            title={`${user.name} · ${ROLE_LABEL[user.role]}`}
            className="shrink-0 flex items-center gap-2 p-0.5 rounded-full bg-white/10"
          >
            <span
              aria-hidden="true"
              className="size-10 grid place-items-center rounded-full bg-kunyit text-ink text-sm font-bold"
            >
              {initials(user.name)}
            </span>
            <p className="text-sm leading-tight hidden sm:block">
              <span className="font-semibold">{user.name}</span>
              <br />
              <span className="text-paper/70">{ROLE_LABEL[user.role]}</span>
            </p>
            <span aria-hidden="true" className="w-px h-6 bg-paper/25" />
            <button
              type="button"
              onClick={() => void handleLogout()}
              title="Keluar"
              aria-label="Keluar"
              className="size-10 grid place-items-center rounded-full hover:bg-merah focus-visible:outline-paper cursor-pointer"
            >
              <LogoutIcon />
            </button>
          </div>
        </div>

        {menuOpen && (
          <nav
            id={menuId}
            aria-label="Menu utama"
            className="lg:hidden absolute inset-x-0 top-full bg-ink border-t border-paper/15 shadow-lg"
          >
            <ul className="mx-auto max-w-6xl p-2 grid gap-1 sm:grid-cols-2">
              {items.map((item) => {
                const count = badge[item.to] ?? 0
                return (
                  <li key={item.to}>
                    <NavLink
                      to={item.to}
                      onClick={() => setMenuOpen(false)}
                      className={({ isActive }) =>
                        `h-12 px-3 flex items-center justify-between gap-2 rounded-sm font-semibold focus-visible:outline-paper ${
                          isActive ? 'bg-kunyit text-ink' : 'text-paper hover:bg-white/10'
                        }`
                      }
                    >
                      {item.label}
                      {count > 0 && <CountBadge count={count} />}
                    </NavLink>
                  </li>
                )
              })}
            </ul>
            <p className="sm:hidden px-5 pb-3 text-sm text-paper/70">
              Masuk sebagai <span className="font-semibold text-paper">{user.name}</span> · {ROLE_LABEL[user.role]}
            </p>
          </nav>
        )}
      </header>

      {menuOpen && (
        <div aria-hidden="true" onClick={() => setMenuOpen(false)} className="lg:hidden fixed inset-0 z-30 bg-ink/40" />
      )}

      {(urgentCount(preorders) > 0 || urgentCount(deliveries) > 0) && (
        <div className="px-4 py-2 flex flex-wrap sm:justify-center gap-x-6 gap-y-2 bg-paper border-b border-line text-sm sm:text-base font-semibold">
          {preorders && urgentCount(preorders) > 0 && (
            <SummaryLink to="/preorder" label="Pre-order" summary={preorders} />
          )}
          {deliveries && urgentCount(deliveries) > 0 && (
            <SummaryLink to="/pengiriman" label="Pengiriman" summary={deliveries} />
          )}
        </div>
      )}

      <main className="flex-1 mx-auto w-full max-w-6xl p-4">
        <Outlet />
      </main>
    </div>
  )
}
