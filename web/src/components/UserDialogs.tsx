import { useState, type FormEvent, type ReactNode } from 'react'
import { ApiError } from '../api/client'
import { createUser, updateUser } from '../api/users'
import type { Role, User } from '../api/types'
import { ROLE_LABEL } from '../lib/user'
import { Modal } from './Modal'
import { PasswordField } from './PasswordField'

const inputClass = 'w-full h-11 px-3 bg-white border-2 border-ink rounded-sm'
const labelClass = 'block mb-1 text-sm font-semibold'
const cancelClass = 'h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer'
const submitClass =
  'h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait'

const message = (e: unknown) => (e instanceof ApiError ? e.message : 'Terjadi kesalahan. Coba lagi.')

function ErrorLine({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="font-semibold text-merah">
      {children}
    </p>
  )
}

// Ditampilkan setelah password dibuat atau diganti, karena password tidak bisa dilihat lagi setelah dialog ditutup.
function Credentials({ rows, onDone }: { rows: [string, string][]; onDone: () => void }) {
  return (
    <div className="mt-4 space-y-4">
      <dl className="p-3 bg-wall font-mono space-y-1">
        {rows.map(([term, value]) => (
          <div key={term} className="flex gap-3">
            <dt className="w-24 shrink-0 text-ink-soft">{term}</dt>
            <dd className="font-semibold break-all">{value}</dd>
          </div>
        ))}
      </dl>
      <p className="text-sm text-ink-soft">
        Catat password ini dan berikan ke yang bersangkutan. Setelah dialog ditutup, password tidak bisa dilihat lagi.
      </p>
      <div className="flex justify-end">
        <button type="button" onClick={onDone} className={submitClass}>
          Selesai
        </button>
      </div>
    </div>
  )
}

export function CreateUserDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('cashier')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [created, setCreated] = useState<{ user: User; password: string } | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const cleanName = name.trim()
    const cleanUsername = username.trim()

    if (!cleanName) return setError('Nama wajib diisi.')
    if (!/^[A-Za-z0-9]{3,50}$/.test(cleanUsername)) {
      return setError('Username 3 sampai 50 karakter, hanya huruf dan angka.')
    }
    if (password.length < 8) return setError('Password minimal 8 karakter.')

    setBusy(true)
    setError('')
    try {
      const user = await createUser({ name: cleanName, username: cleanUsername, password, role })
      setCreated({ user, password })
      onCreated()
    } catch (err) {
      setError(message(err))
      setBusy(false)
    }
  }

  if (created) {
    return (
      <Modal title="Akun dibuat" onClose={onClose}>
        <p className="mt-2 font-semibold text-hijau">
          {created.user.name} ({ROLE_LABEL[created.user.role]}) bisa masuk dengan data ini.
        </p>
        <Credentials
          rows={[
            ['Username', created.user.username],
            ['Password', created.password],
          ]}
          onDone={onClose}
        />
      </Modal>
    )
  }

  return (
    <Modal title="Tambah pengguna" onClose={onClose} busy={busy}>
      <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-4">
        <div>
          <label htmlFor="u-name" className={labelClass}>
            Nama
          </label>
          <input
            id="u-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
            maxLength={100}
            className={inputClass}
          />
        </div>
        <div>
          <label htmlFor="u-username" className={labelClass}>
            Username
          </label>
          <input
            id="u-username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            maxLength={50}
            className={`${inputClass} font-mono`}
          />
          <p className="mt-1 text-sm text-ink-soft">Huruf dan angka saja, tanpa spasi.</p>
        </div>
        <PasswordField id="u-password" label="Password" value={password} onChange={setPassword} />
        <div>
          <label htmlFor="u-role" className={labelClass}>
            Peran
          </label>
          <select id="u-role" value={role} onChange={(e) => setRole(e.target.value as Role)} className={inputClass}>
            <option value="cashier">Kasir (hanya menjual dan melihat transaksinya sendiri)</option>
            <option value="admin">Admin (akses penuh)</option>
          </select>
          <p className="mt-1 text-sm text-ink-soft">Peran tidak bisa diubah setelah akun dibuat.</p>
        </div>

        {error && <ErrorLine>{error}</ErrorLine>}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" onClick={onClose} className={cancelClass}>
            Batal
          </button>
          <button type="submit" disabled={busy} className={submitClass}>
            Tambah pengguna
          </button>
        </div>
      </form>
    </Modal>
  )
}

export function RenameUserDialog({
  user,
  onClose,
  onSaved,
}: {
  user: User
  onClose: () => void
  onSaved: () => void
}) {
  const [name, setName] = useState(user.name)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const clean = name.trim()
    if (!clean) return setError('Nama wajib diisi.')

    setBusy(true)
    setError('')
    try {
      await updateUser(user.id, { name: clean })
      onSaved()
    } catch (err) {
      setError(message(err))
      setBusy(false)
    }
  }

  return (
    <Modal title="Ubah nama" onClose={onClose} busy={busy}>
      <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-4">
        <div>
          <label htmlFor="rn-name" className={labelClass}>
            Nama untuk <span className="font-mono">{user.username}</span>
          </label>
          <input
            id="rn-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
            maxLength={100}
            className={inputClass}
          />
        </div>

        {error && <ErrorLine>{error}</ErrorLine>}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" onClick={onClose} className={cancelClass}>
            Batal
          </button>
          <button type="submit" disabled={busy} className={submitClass}>
            Simpan nama
          </button>
        </div>
      </form>
    </Modal>
  )
}

export function ResetPasswordDialog({
  user,
  isSelf,
  onClose,
  onSaved,
}: {
  user: User
  isSelf: boolean
  onClose: () => void
  onSaved: () => void
}) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (password.length < 8) return setError('Password minimal 8 karakter.')

    setBusy(true)
    setError('')
    try {
      await updateUser(user.id, { password })
      setDone(password)
      onSaved()
    } catch (err) {
      setError(message(err))
      setBusy(false)
    }
  }

  if (done !== null) {
    return (
      <Modal title="Password diganti" onClose={onClose}>
        <p className="mt-2 font-semibold text-hijau">Password {user.name} sudah diganti.</p>
        <Credentials
          rows={[
            ['Username', user.username],
            ['Password', done],
          ]}
          onDone={onClose}
        />
      </Modal>
    )
  }

  return (
    <Modal title="Reset password" onClose={onClose} busy={busy}>
      <p className="mt-2 text-ink-soft">
        Password baru untuk <span className="font-semibold text-ink">{user.name}</span>. Semua sesi login pengguna ini
        berakhir.
        {isSelf && ' Karena ini akunmu sendiri, kamu akan diminta masuk lagi dalam beberapa menit.'}
      </p>
      <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-4">
        <PasswordField id="rp-password" label="Password baru" value={password} onChange={setPassword} />

        {error && <ErrorLine>{error}</ErrorLine>}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" onClick={onClose} className={cancelClass}>
            Batal
          </button>
          <button type="submit" disabled={busy} className={submitClass}>
            Ganti password
          </button>
        </div>
      </form>
    </Modal>
  )
}
