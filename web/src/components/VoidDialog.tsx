import { useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { voidTransaction } from '../api/transactions'
import type { Transaction } from '../api/types'
import { formatRupiah } from '../lib/format'
import { Modal } from './Modal'

export function VoidDialog({
  transaction,
  onClose,
  onVoided,
}: {
  transaction: Transaction
  onClose: () => void
  onVoided: (updated: Transaction) => void
}) {
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const text = reason.trim()
    if (!text) return setError('Alasan pembatalan wajib diisi.')

    setBusy(true)
    setError('')
    try {
      onVoided(await voidTransaction(transaction.id, text))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Terjadi kesalahan. Coba lagi.')
      setBusy(false)
    }
  }

  return (
    <Modal title="Batalkan transaksi?" onClose={onClose} busy={busy}>
      <p className="mt-2 text-ink-soft">
        Struk <span className="font-mono font-semibold text-ink">{transaction.receipt_no}</span> senilai{' '}
        <span className="font-mono font-semibold text-ink">{formatRupiah(transaction.total)}</span> akan dibatalkan dan
        stok barangnya dikembalikan. Pembatalan tidak bisa diurungkan, dan uang pelanggan dikembalikan secara manual.
      </p>

      <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-4">
        <div>
          <label htmlFor="void-reason" className="block mb-1 text-sm font-semibold">
            Alasan pembatalan
          </label>
          <input
            id="void-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={200}
            autoFocus
            autoComplete="off"
            placeholder="Contoh: salah input jumlah"
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
            Batalkan transaksi
          </button>
        </div>
      </form>
    </Modal>
  )
}
