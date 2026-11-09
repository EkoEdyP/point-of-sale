import { useState, type ReactNode } from 'react'
import { ApiError } from '../api/client'
import { Modal } from './Modal'

export function ConfirmDialog({
  title,
  confirmLabel,
  onConfirm,
  onClose,
  children,
}: {
  title: string
  confirmLabel: string
  onConfirm: () => Promise<void>
  onClose: () => void
  children: ReactNode
}) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function handleConfirm() {
    setBusy(true)
    setError('')
    try {
      await onConfirm()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Terjadi kesalahan. Coba lagi.')
      setBusy(false)
    }
  }

  return (
    <Modal title={title} onClose={onClose} busy={busy}>
      <div className="mt-2 text-ink-soft">{children}</div>
      {error && (
        <p role="alert" className="mt-3 font-semibold text-merah">
          {error}
        </p>
      )}
      <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button
          type="button"
          onClick={onClose}
          className="h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer"
        >
          Batal
        </button>
        <button
          type="button"
          onClick={() => void handleConfirm()}
          disabled={busy}
          className="h-11 px-4 bg-merah text-white rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait"
        >
          {confirmLabel}
        </button>
      </div>
    </Modal>
  )
}
