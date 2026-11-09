import { useState, type FormEvent, type ReactNode } from 'react'
import { ApiError } from '../api/client'
import { Modal } from './Modal'

// Dialog yang meminta alasan sebelum tindakan yang tidak bisa diurungkan (misalnya membatalkan pre-order).
export function ReasonDialog({
  title,
  description,
  confirmLabel,
  onConfirm,
  onClose,
}: {
  title: string
  description: ReactNode
  confirmLabel: string
  onConfirm: (reason: string) => Promise<void>
  onClose: () => void
}) {
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const text = reason.trim()
    if (!text) return setError('Alasan wajib diisi.')

    setBusy(true)
    setError('')
    try {
      await onConfirm(text)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Terjadi kesalahan. Coba lagi.')
      setBusy(false)
    }
  }

  return (
    <Modal title={title} onClose={onClose} busy={busy}>
      <div className="mt-2 text-ink-soft">{description}</div>
      <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-4">
        <div>
          <label htmlFor="reason" className="block mb-1 text-sm font-semibold">
            Alasan
          </label>
          <input
            id="reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={200}
            autoFocus
            autoComplete="off"
            className="w-full h-11 px-3 bg-white border-2 border-ink rounded-sm"
          />
        </div>

        {error && (
          <p role="alert" className="font-semibold text-merah">
            {error}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            className="h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer"
          >
            Kembali
          </button>
          <button
            type="submit"
            disabled={busy}
            className="h-11 px-4 bg-merah text-white rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait"
          >
            {confirmLabel}
          </button>
        </div>
      </form>
    </Modal>
  )
}
