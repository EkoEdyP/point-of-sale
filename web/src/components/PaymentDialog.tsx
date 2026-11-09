import { useState, type FormEvent, type ReactNode } from 'react'
import { ApiError } from '../api/client'
import type { PaymentMethod } from '../api/types'
import { formatNumber, formatRupiah } from '../lib/format'
import { PAYMENT_METHODS, cashSuggestions } from '../lib/payment'
import { Modal } from './Modal'
import { ReceiverField } from './ReceiverField'

const digitsOnly = (value: string) => value.replace(/\D/g, '').slice(0, 12)

// note: keterangan di bawah total, misalnya apa saja yang ikut selesai setelah dibayar. receiver: minta nama penerima
// barang (pre-order diantar yang ikut ditandai terkirim); onPay menerimanya sebagai receivedBy.
export function PaymentDialog({
  total,
  onPay,
  onClose,
  title = 'Pembayaran',
  totalLabel = 'Total belanja',
  submitLabel = 'Selesaikan pembayaran',
  note,
  receiver,
}: {
  total: number
  onPay: (method: PaymentMethod, paid: number, receivedBy: string) => Promise<void>
  onClose: () => void
  title?: string
  totalLabel?: string
  submitLabel?: string
  note?: ReactNode
  receiver?: { customerName: string }
}) {
  const [method, setMethod] = useState<PaymentMethod>('cash')
  const [received, setReceived] = useState('')
  const [receivedBy, setReceivedBy] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const isCash = method === 'cash'
  const paid = isCash ? Number(received || '0') : total
  const shortBy = total - paid
  const change = paid - total

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (busy || (isCash && shortBy > 0)) return
    if (receiver && !receivedBy.trim()) return setError('Nama penerima wajib diisi.')
    setBusy(true)
    setError('')
    try {
      await onPay(method, paid, receivedBy.trim())
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Terjadi kesalahan. Coba lagi.')
      setBusy(false)
    }
  }

  return (
    <Modal title={title} onClose={onClose} busy={busy}>
      <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-5">
        <div>
          <p className="text-sm font-semibold text-ink-soft">{totalLabel}</p>
          <p className="font-mono text-3xl sm:text-4xl font-semibold">{formatRupiah(total)}</p>
          {note && <p className="mt-1 text-sm text-ink-soft">{note}</p>}
        </div>

        {receiver && (
          <ReceiverField value={receivedBy} onChange={setReceivedBy} customerName={receiver.customerName} autoFocus />
        )}

        <div role="radiogroup" aria-label="Metode pembayaran" className="grid grid-cols-2 sm:grid-cols-4 gap-2">
          {PAYMENT_METHODS.map((m) => (
            <button
              key={m.value}
              type="button"
              role="radio"
              aria-checked={method === m.value}
              onClick={() => setMethod(m.value)}
              className={`h-12 border-2 border-ink rounded-sm text-sm font-bold cursor-pointer ${
                method === m.value ? 'bg-ink text-paper' : 'bg-paper'
              }`}
            >
              {m.label}
            </button>
          ))}
        </div>

        {isCash ? (
          <div className="space-y-3">
            <div>
              <label htmlFor="received" className="block mb-1 text-sm font-semibold">
                Uang diterima (Rp)
              </label>
              <input
                id="received"
                inputMode="numeric"
                autoFocus={!receiver}
                autoComplete="off"
                value={received ? formatNumber(Number(received)) : ''}
                onChange={(e) => setReceived(digitsOnly(e.target.value))}
                className="w-full h-14 px-3 bg-white border-2 border-ink rounded-sm font-mono text-2xl"
              />
            </div>

            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => setReceived(String(total))}
                className="h-10 px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer"
              >
                Uang pas
              </button>
              {cashSuggestions(total).map((amount) => (
                <button
                  key={amount}
                  type="button"
                  onClick={() => setReceived(String(amount))}
                  className="h-10 px-3 border-2 border-ink rounded-sm font-mono font-semibold cursor-pointer"
                >
                  {formatNumber(amount)}
                </button>
              ))}
            </div>

            <p className="font-mono text-xl font-semibold" aria-live="polite">
              {received === '' ? (
                <span className="text-ink-soft">Kembalian Rp 0</span>
              ) : shortBy > 0 ? (
                <span className="text-merah">Kurang {formatRupiah(shortBy)}</span>
              ) : (
                <span className="text-hijau">Kembalian {formatRupiah(change)}</span>
              )}
            </p>
          </div>
        ) : (
          <p className="text-ink-soft">
            Terima pembayaran {formatRupiah(total)} lewat{' '}
            {PAYMENT_METHODS.find((m) => m.value === method)?.label}, lalu selesaikan transaksi.
          </p>
        )}

        {error && (
          <p role="alert" className="font-semibold text-merah">
            {error}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            className="h-12 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer"
          >
            Batal
          </button>
          <button
            type="submit"
            disabled={busy || (isCash && shortBy > 0)}
            className="h-12 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {busy ? 'Menyimpan…' : submitLabel}
          </button>
        </div>
      </form>
    </Modal>
  )
}
