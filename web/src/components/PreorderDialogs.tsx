import { useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { addDownPayment, updatePreorder, type PreorderPatch } from '../api/preorders'
import type { PaymentMethod, Preorder } from '../api/types'
import { usePreorderQuota } from '../hooks/usePreorderQuota'
import { isActiveDelivery } from '../lib/delivery'
import { formatNumber, formatRupiah, toDateInput } from '../lib/format'
import { PAYMENT_METHODS } from '../lib/payment'
import { MAX_PREORDER_FEE } from '../lib/preorder'
import { withRequestKey } from '../lib/requestKey'
import { isStale, staleMessage } from '../lib/version'
import { Modal } from './Modal'
import { QuotaRemaining, quotaProblem } from './PreorderQuota'

const inputClass = 'w-full h-11 px-3 bg-white border-2 border-ink rounded-sm'
const labelClass = 'block mb-1 text-sm font-semibold'
const cancelClass = 'h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer'
const submitClass =
  'h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait'

const digitsOnly = (value: string) => value.replace(/\D/g, '').slice(0, 12)
const message = (e: unknown) => (e instanceof ApiError ? e.message : 'Terjadi kesalahan. Coba lagi.')

export function AddDownPaymentDialog({
  preorder,
  onClose,
  onSaved,
}: {
  preorder: Preorder
  onClose: () => void
  onSaved: () => void
}) {
  const [amount, setAmount] = useState('')
  const [method, setMethod] = useState<PaymentMethod>('cash')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const value = Number(amount || '0')
    if (value <= 0) return setError('Jumlah DP wajib diisi.')
    if (value > preorder.remaining) {
      return setError(`DP melebihi sisa tagihan (${formatRupiah(preorder.remaining)}).`)
    }

    setBusy(true)
    setError('')
    try {
      // Kunci per pre-order: DP yang sama dikirim ulang (juga setelah dialog ditutup) tidak tercatat dua kali.
      await withRequestKey(`dp:${preorder.id}`, (key) => addDownPayment(preorder.id, value, method, key))
      onSaved()
    } catch (err) {
      setError(message(err))
      setBusy(false)
    }
  }

  return (
    <Modal title="Tambah DP" onClose={onClose} busy={busy}>
      <p className="mt-2 text-ink-soft">
        Sisa tagihan <span className="font-mono font-semibold text-ink">{formatRupiah(preorder.remaining)}</span>
      </p>
      <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-4">
        <div>
          <label htmlFor="dp-amount" className={labelClass}>
            Jumlah DP (Rp)
          </label>
          <input
            id="dp-amount"
            inputMode="numeric"
            autoFocus
            autoComplete="off"
            value={amount ? formatNumber(Number(amount)) : ''}
            onChange={(e) => setAmount(digitsOnly(e.target.value))}
            className={`${inputClass} font-mono text-lg`}
          />
        </div>
        <div>
          <label htmlFor="dp-method" className={labelClass}>
            Metode bayar
          </label>
          <select
            id="dp-method"
            value={method}
            onChange={(e) => setMethod(e.target.value as PaymentMethod)}
            className={inputClass}
          >
            {PAYMENT_METHODS.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </select>
        </div>

        {error && (
          <p role="alert" className="font-semibold text-merah">
            {error}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" onClick={onClose} className={cancelClass}>
            Batal
          </button>
          <button type="submit" disabled={busy} className={submitClass}>
            Catat DP
          </button>
        </div>
      </form>
    </Modal>
  )
}

export function ScheduleDialog({
  preorder,
  onClose,
  onSaved,
}: {
  preorder: Preorder
  onClose: () => void
  onSaved: () => void
}) {
  const [date, setDate] = useState(preorder.due_date)
  const [time, setTime] = useState(preorder.due_time ?? '')
  const [notes, setNotes] = useState(preorder.notes ?? '')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  // Versi saat dialog dibuka; detail yang dimuat ulang di belakang dialog tidak menggantinya.
  const [version] = useState(preorder.version)
  const [stale, setStale] = useState(false)
  // Setelah ditolak karena datanya sudah diubah orang lain, menutup dialog sekaligus memuat ulang detailnya.
  const close = stale ? onSaved : onClose
  // Kuota hanya dicek kalau hari H dipindah: seluruh pcs pesanan ini harus muat di tanggal yang baru.
  const moving = date !== preorder.due_date
  const pieces = (preorder.items ?? []).reduce((sum, item) => sum + item.qty, 0)
  const quota = usePreorderQuota(moving ? date : '', preorder.id)
  const overQuota = moving ? quotaProblem(quota, pieces) : ''
  // Pernah ditolak karena kuota: pesannya ikut tampil di dekat tombol, dan hilang sendiri setelah diperbaiki.
  const [quotaBlocked, setQuotaBlocked] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (stale) return
    if (!date) return setError('Tanggal wajib diisi.')
    if (overQuota) return setQuotaBlocked(true)

    // Tanggal hanya dikirim kalau diubah: pre-order yang sudah terlambat tetap bisa diubah catatannya.
    const patch: { due_date?: string; due_time: string; notes: string } = { due_time: time, notes: notes.trim() }
    if (date !== preorder.due_date) patch.due_date = date

    setBusy(true)
    setError('')
    try {
      await updatePreorder(preorder.id, { ...patch, version })
      onSaved()
    } catch (err) {
      setStale(isStale(err))
      setError(isStale(err) ? staleMessage('Pre-order') : message(err))
      setBusy(false)
    }
  }

  return (
    <Modal title="Ubah jadwal dan catatan" onClose={close} busy={busy}>
      {isActiveDelivery(preorder.delivery_status) && (
        <p className="mt-2 text-ink-soft">
          Jadwal dan catatan pengiriman{' '}
          <span className="font-mono font-semibold text-ink">{preorder.delivery_no}</span> ikut berubah.
        </p>
      )}
      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="mt-4 space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label htmlFor="sc-date" className={labelClass}>
              Tanggal (hari H)
            </label>
            <input
              id="sc-date"
              type="date"
              min={toDateInput(new Date())}
              value={date}
              onChange={(e) => setDate(e.target.value)}
              aria-describedby="sc-date-quota"
              className={inputClass}
            />
            {moving && <QuotaRemaining quota={quota} id="sc-date-quota" />}
            {overQuota && quota && quota.remaining > 0 && (
              <p className="mt-1 text-sm font-semibold text-merah">{overQuota}</p>
            )}
          </div>
          <div>
            <label htmlFor="sc-time" className={labelClass}>
              Jam (opsional)
            </label>
            <input
              id="sc-time"
              type="time"
              value={time}
              onChange={(e) => setTime(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>
        <div>
          <label htmlFor="sc-notes" className={labelClass}>
            Catatan
          </label>
          <textarea
            id="sc-notes"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            maxLength={500}
            rows={3}
            className="w-full px-3 py-2 bg-white border-2 border-ink rounded-sm"
          />
        </div>

        {(error || (quotaBlocked && overQuota)) && (
          <p role="alert" className="font-semibold text-merah">
            {error || overQuota}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" onClick={close} className={cancelClass}>
            Batal
          </button>
          {stale ? (
            <button type="button" onClick={close} className={submitClass}>
              Muat ulang
            </button>
          ) : (
            <button type="submit" disabled={busy} className={submitClass}>
              Simpan
            </button>
          )}
        </div>
      </form>
    </Modal>
  )
}

// Nama, No. HP, dan alamat pelanggan. Pengiriman yang masih berjalan ikut berubah, karena pemesan dan alamatnya
// selalu sama dengan pre-order-nya.
export function CustomerDialog({
  preorder,
  onClose,
  onSaved,
}: {
  preorder: Preorder
  onClose: () => void
  onSaved: () => void
}) {
  const delivery = preorder.fulfillment === 'delivery'
  const [name, setName] = useState(preorder.customer_name)
  const [phone, setPhone] = useState(preorder.customer_phone ?? '')
  const [address, setAddress] = useState(preorder.address ?? '')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  // Seperti ScheduleDialog: versi saat dibuka, dan setelah ditolak karena data lama hanya bisa muat ulang.
  const [version] = useState(preorder.version)
  const [stale, setStale] = useState(false)
  const close = stale ? onSaved : onClose

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (stale) return
    if (!name.trim()) return setError('Nama pelanggan wajib diisi.')
    if (delivery && !address.trim()) return setError('Alamat pengantaran wajib diisi.')

    const patch: PreorderPatch = {}
    if (name.trim() !== preorder.customer_name) patch.customer_name = name.trim()
    if (phone.trim() !== (preorder.customer_phone ?? '')) patch.customer_phone = phone.trim()
    if (delivery && address.trim() !== (preorder.address ?? '')) patch.address = address.trim()
    if (Object.keys(patch).length === 0) return onClose()

    setBusy(true)
    setError('')
    try {
      await updatePreorder(preorder.id, { ...patch, version })
      onSaved()
    } catch (err) {
      setStale(isStale(err))
      setError(isStale(err) ? staleMessage('Pre-order') : message(err))
      setBusy(false)
    }
  }

  return (
    <Modal title="Ubah data pelanggan" onClose={close} busy={busy}>
      {isActiveDelivery(preorder.delivery_status) && (
        <p className="mt-2 text-ink-soft">
          Pemesan dan alamat di pengiriman{' '}
          <span className="font-mono font-semibold text-ink">{preorder.delivery_no}</span> ikut berubah.
        </p>
      )}
      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="mt-4 space-y-4">
        <div className="grid gap-x-3 gap-y-4 sm:gap-y-3 sm:grid-cols-2">
          <div>
            <label htmlFor="cu-name" className={labelClass}>
              Nama pelanggan
            </label>
            <input
              id="cu-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={100}
              autoFocus
              className={inputClass}
            />
          </div>
          <div>
            <label htmlFor="cu-phone" className={labelClass}>
              No. HP (opsional)
            </label>
            <input
              id="cu-phone"
              type="tel"
              inputMode="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              maxLength={30}
              className={`${inputClass} font-mono`}
            />
          </div>
        </div>
        {delivery && (
          <div>
            <label htmlFor="cu-address" className={labelClass}>
              Alamat pengantaran
            </label>
            <textarea
              id="cu-address"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              maxLength={300}
              rows={2}
              className="w-full px-3 py-2 bg-white border-2 border-ink rounded-sm"
            />
          </div>
        )}

        {error && (
          <p role="alert" className="font-semibold text-merah">
            {error}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" onClick={close} className={cancelClass}>
            Batal
          </button>
          {stale ? (
            <button type="button" onClick={close} className={submitClass}>
              Muat ulang
            </button>
          ) : (
            <button type="submit" disabled={busy} className={submitClass}>
              Simpan
            </button>
          )}
        </div>
      </form>
    </Modal>
  )
}

// Ongkir dan biaya lain bisa diubah selama pre-order berjalan, mis. ongkir baru pasti setelah kurir menghitung
// atau kresek ditambahkan saat diambil. Total dan sisa tagihan ikut berubah.
export function FeesDialog({
  preorder,
  onClose,
  onSaved,
}: {
  preorder: Preorder
  onClose: () => void
  onSaved: () => void
}) {
  const delivery = preorder.fulfillment === 'delivery'
  const [shipping, setShipping] = useState(preorder.shipping_fee ? String(preorder.shipping_fee) : '')
  const [other, setOther] = useState(preorder.other_fee ? String(preorder.other_fee) : '')
  const [note, setNote] = useState(preorder.other_fee_note ?? '')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  // Seperti ScheduleDialog: versi saat dibuka, dan setelah ditolak karena data lama hanya bisa muat ulang.
  const [version] = useState(preorder.version)
  const [stale, setStale] = useState(false)
  const close = stale ? onSaved : onClose

  const ongkir = delivery ? Number(shipping || '0') : 0
  const lain = Number(other || '0')
  const total = preorder.subtotal + ongkir + lain
  const rows: [string, number][] = [['Subtotal barang', preorder.subtotal], ['Biaya lain', lain]]
  if (delivery) rows.push(['Ongkir', ongkir])
  rows.push(['Total', total], ['DP diterima', preorder.down_payment])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (stale) return
    if (ongkir > MAX_PREORDER_FEE || lain > MAX_PREORDER_FEE) {
      return setError(`Ongkir dan biaya lain maksimal ${formatRupiah(MAX_PREORDER_FEE)}.`)
    }
    if (total < preorder.down_payment) {
      return setError(`Total tidak boleh kurang dari DP yang sudah diterima (${formatRupiah(preorder.down_payment)}).`)
    }

    const patch: PreorderPatch = { other_fee: lain, other_fee_note: lain > 0 ? note.trim() : '', version }
    if (delivery) patch.shipping_fee = ongkir

    setBusy(true)
    setError('')
    try {
      await updatePreorder(preorder.id, patch)
      onSaved()
    } catch (err) {
      setStale(isStale(err))
      setError(isStale(err) ? staleMessage('Pre-order') : message(err))
      setBusy(false)
    }
  }

  return (
    <Modal title={delivery ? 'Ubah ongkir dan biaya lain' : 'Ubah biaya lain'} onClose={close} busy={busy}>
      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="mt-4 space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          {delivery && (
            <div>
              <label htmlFor="fe-shipping" className={labelClass}>
                Ongkir (Rp)
              </label>
              <input
                id="fe-shipping"
                inputMode="numeric"
                autoComplete="off"
                value={shipping ? formatNumber(Number(shipping)) : ''}
                onChange={(e) => setShipping(digitsOnly(e.target.value))}
                className={`${inputClass} font-mono`}
              />
            </div>
          )}
          <div>
            <label htmlFor="fe-other" className={labelClass}>
              Biaya lain (Rp)
            </label>
            <input
              id="fe-other"
              inputMode="numeric"
              autoComplete="off"
              value={other ? formatNumber(Number(other)) : ''}
              onChange={(e) => setOther(digitsOnly(e.target.value))}
              className={`${inputClass} font-mono`}
            />
          </div>
          <div className={delivery ? 'sm:col-span-2' : ''}>
            <label htmlFor="fe-note" className={labelClass}>
              Keterangan biaya lain
            </label>
            <input
              id="fe-note"
              placeholder="mis. kresek"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              disabled={lain === 0}
              maxLength={40}
              className={`${inputClass} disabled:opacity-50`}
            />
          </div>
        </div>

        <dl className="font-mono space-y-1">
          {rows.map(([label, value]) => (
            <div key={label} className="flex justify-between gap-3">
              <dt>{label}</dt>
              <dd>{formatRupiah(value)}</dd>
            </div>
          ))}
          <div className="flex justify-between gap-3 pt-1 border-t-2 border-dashed border-ink text-lg">
            <dt className="font-semibold">Sisa tagihan</dt>
            <dd className="font-semibold">{formatRupiah(Math.max(total - preorder.down_payment, 0))}</dd>
          </div>
        </dl>

        {error && (
          <p role="alert" className="font-semibold text-merah">
            {error}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" onClick={close} className={cancelClass}>
            Batal
          </button>
          {stale ? (
            <button type="button" onClick={close} className={submitClass}>
              Muat ulang
            </button>
          ) : (
            <button type="submit" disabled={busy} className={submitClass}>
              Simpan
            </button>
          )}
        </div>
      </form>
    </Modal>
  )
}
