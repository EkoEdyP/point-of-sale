import { useEffect, useState } from 'react'
import { ApiError } from '../api/client'
import { listUsers, updateUser } from '../api/users'
import type { User } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { AuditHistoryDialog } from '../components/AuditHistoryDialog'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { CreateUserDialog, RenameUserDialog, ResetPasswordDialog } from '../components/UserDialogs'
import { ROLE_LABEL } from '../lib/user'

const rowButton = 'h-11 md:pointer-fine:h-9 px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer disabled:opacity-50'
const dangerButton = 'h-11 md:pointer-fine:h-9 px-3 border-2 border-merah text-merah rounded-sm font-semibold cursor-pointer'

function SelfBadge() {
  return <span className="ml-2 px-1.5 py-0.5 text-xs font-semibold border border-ink rounded-sm">Kamu</span>
}

function InactiveBadge() {
  return (
    <span className="px-1.5 py-0.5 text-xs font-semibold border border-merah text-merah rounded-sm">Nonaktif</span>
  )
}

export function UsersPage() {
  const { state } = useAuth()
  const meId = state.status === 'authenticated' ? state.user.id : null

  const [users, setUsers] = useState<User[] | null>(null)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const [busyId, setBusyId] = useState<string | null>(null)

  const [creating, setCreating] = useState(false)
  const [renaming, setRenaming] = useState<User | null>(null)
  const [resetting, setResetting] = useState<User | null>(null)
  const [deactivating, setDeactivating] = useState<User | null>(null)
  const [showingHistory, setShowingHistory] = useState<User | null>(null)

  useEffect(() => {
    let cancelled = false
    listUsers()
      .then((r) => {
        if (cancelled) return
        setUsers(r.data)
        setError('')
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat pengguna.')
      })
    return () => {
      cancelled = true
    }
  }, [reloadKey])

  const reload = () => setReloadKey((k) => k + 1)

  // Tombol aksi per akun, dipakai di tabel (tablet/desktop) dan di kartu (HP).
  const actions = (u: User) => (
    <>
      <button type="button" onClick={() => setRenaming(u)} className={rowButton}>
        Ubah nama
      </button>
      <button type="button" onClick={() => setResetting(u)} className={rowButton}>
        Reset password
      </button>
      <button type="button" onClick={() => setShowingHistory(u)} className={rowButton}>
        Riwayat
      </button>
      {u.active ? (
        u.id !== meId && (
          <button type="button" onClick={() => setDeactivating(u)} className={dangerButton}>
            Nonaktifkan
          </button>
        )
      ) : (
        <button type="button" disabled={busyId === u.id} onClick={() => void activate(u)} className={rowButton}>
          Aktifkan
        </button>
      )}
    </>
  )

  async function activate(user: User) {
    setBusyId(user.id)
    setError('')
    try {
      await updateUser(user.id, { active: true })
      reload()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Gagal mengaktifkan akun.')
    } finally {
      setBusyId(null)
    }
  }

  return (
    <section>
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-bold mr-auto">Pengguna</h1>
        <button
          type="button"
          onClick={() => setCreating(true)}
          className="h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer"
        >
          Tambah pengguna
        </button>
      </div>
      <p className="mt-1 text-ink-soft">
        Akun yang dinonaktifkan tidak bisa masuk dan sesinya langsung berakhir. Riwayat transaksinya tetap ada.
      </p>

      {error && (
        <p role="alert" className="mt-4 font-semibold text-merah">
          {error}
        </p>
      )}

      <div className="mt-4 overflow-x-auto bg-paper border border-line">
        {/* Tablet dan layar lebar: tabel. */}
        <table className="hidden md:table w-full min-w-[40rem] text-left">
          <thead>
            <tr className="border-b-2 border-ink text-sm text-ink-soft">
              <th className="px-4 py-3 font-semibold">Nama</th>
              <th className="px-4 py-3 font-semibold">Username</th>
              <th className="px-4 py-3 font-semibold">Peran</th>
              <th className="px-4 py-3 font-semibold">Status</th>
              <th className="px-4 py-3 font-semibold">
                <span className="sr-only">Aksi</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {(users ?? []).map((u) => (
              <tr key={u.id} className="border-b border-line hover:bg-wall">
                <td className={`px-4 py-3 font-semibold ${u.active ? '' : 'text-ink-soft'}`}>
                  {u.name}
                  {u.id === meId && <SelfBadge />}
                </td>
                <td className="px-4 py-3 font-mono text-sm">{u.username}</td>
                <td className="px-4 py-3">{ROLE_LABEL[u.role]}</td>
                <td className="px-4 py-3">{u.active ? <span className="text-sm">Aktif</span> : <InactiveBadge />}</td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap justify-end gap-2">{actions(u)}</div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        {/* HP: satu kartu per akun, tombol selebar kartu supaya mudah diketuk. */}
        <ul className="md:hidden divide-y divide-line">
          {(users ?? []).map((u) => (
            <li key={u.id} className="px-4 py-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className={`font-semibold break-words ${u.active ? '' : 'text-ink-soft'}`}>
                    {u.name}
                    {u.id === meId && <SelfBadge />}
                  </p>
                  <p className="font-mono text-sm text-ink-soft break-all">{u.username}</p>
                </div>
                <div className="shrink-0 text-right text-sm">
                  <p>{ROLE_LABEL[u.role]}</p>
                  <p className="mt-0.5">{u.active ? <span className="text-ink-soft">Aktif</span> : <InactiveBadge />}</p>
                </div>
              </div>
              <div className="mt-3 flex flex-wrap gap-2 *:flex-1 *:whitespace-nowrap">{actions(u)}</div>
            </li>
          ))}
        </ul>

        {!users && !error && <p className="px-4 py-10 text-center text-ink-soft">Memuat pengguna…</p>}
      </div>

      {creating && <CreateUserDialog onClose={() => setCreating(false)} onCreated={reload} />}

      {renaming && (
        <RenameUserDialog
          user={renaming}
          onClose={() => setRenaming(null)}
          onSaved={() => {
            setRenaming(null)
            reload()
          }}
        />
      )}

      {resetting && (
        <ResetPasswordDialog
          user={resetting}
          isSelf={resetting.id === meId}
          onClose={() => setResetting(null)}
          onSaved={reload}
        />
      )}

      {showingHistory && (
        <AuditHistoryDialog
          entity="user"
          entityId={showingHistory.id}
          title={`Riwayat akun ${showingHistory.name}`}
          onClose={() => setShowingHistory(null)}
        />
      )}

      {deactivating && (
        <ConfirmDialog
          title="Nonaktifkan akun?"
          confirmLabel="Nonaktifkan"
          onClose={() => setDeactivating(null)}
          onConfirm={async () => {
            await updateUser(deactivating.id, { active: false })
            setDeactivating(null)
            reload()
          }}
        >
          <p>
            <span className="font-semibold text-ink">{deactivating.name}</span> tidak bisa masuk lagi dan sesinya
            langsung berakhir. Akun bisa diaktifkan kembali kapan saja.
          </p>
        </ConfirmDialog>
      )}
    </section>
  )
}
