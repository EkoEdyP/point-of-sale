import { useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { dispatchDelivery, updateDelivery } from '../api/deliveries'
import type { Delivery, DeliveryPatch } from '../api/types'
import { usePreorderQuota } from '../hooks/usePreorderQuota'
import { formatNumber, toDateInput } from '../lib/format'
import { isStale, staleMessage } from '../lib/version'
import { Modal } from './Modal'
import { QuotaRemaining, quotaProblem } from './PreorderQuota'

const inputClass = 'w-full h-11 px-3 bg-white border-2 border-ink rounded-sm'
const labelClass = 'block mb-1 text-sm font-semibold'
const cancelClass = 'h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer'
const submitClass =
  'h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait'

const digitsOnly = (value: string) => value.replace(/\D/g, '').slice(0, 9)
const message = (e: unknown) => (e instanceof ApiError ? e.message : 'Terjadi kesalahan. Coba lagi.')

// Mengubah data pengiriman. Untuk pengiriman yang gagal, dialog ini menjadi "jadwalkan ulang":
// tanggal baru wajib dan statusnya kembali menjadi menunggu. Pengiriman pre-order: pemesan, alamat, jadwal, dan
// catatan ikut tersimpan di pre-order-nya.
export function DeliveryEditDialog({
  delivery,
  onClose,
  onSaved,
}: {
  delivery: Delivery
  onClose: () => void
  onSaved: () => void
}) {
  const failed = delivery.status === 'failed'
  // Ongkir pengiriman pre-order tersimpan di pre-ordernya (ikut total yang dibayar pelanggan).
  const fromPreorder = delivery.source_type === 'preorder'
  const [name, setName] = useState(delivery.customer_name)
  const [phone, setPhone] = useState(delivery.customer_phone ?? '')
  const [address, setAddress] = useState(delivery.address)
  const [date, setDate] = useState(delivery.scheduled_date)
  const [time, setTime] = useState(delivery.scheduled_time ?? '')
  const [courier, setCourier] = useState(delivery.courier ?? '')
  const [fee, setFee] = useState(delivery.fee ? String(delivery.fee) : '')
  const [notes, setNotes] = useState(delivery.notes ?? '')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  // Versi saat dialog dibuka; detail yang dimuat ulang di belakang dialog tidak menggantinya.
  const [version] = useState(delivery.version)
  const [stale, setStale] = useState(false)
  // Setelah ditolak karena datanya sudah diubah orang lain, menutup dialog sekaligus memuat ulang detailnya.
  const close = stale ? onSaved : onClose
  // Tanggal kirim pengiriman pre-order = hari H-nya: pindah tanggal dicek kuotanya seperti mengubah hari H.
  const moving = fromPreorder && date !== delivery.scheduled_date
  const pieces = (delivery.items ?? []).reduce((sum, item) => sum + item.qty, 0)
  const quota = usePreorderQuota(moving ? date : '', delivery.source_id)
  const overQuota = moving ? quotaProblem(quota, pieces) : ''
  // Pernah ditolak karena kuota: pesannya ikut tampil di dekat tombol, dan hilang sendiri setelah diperbaiki.
  const [quotaBlocked, setQuotaBlocked] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (stale) return
    if (!name.trim()) return setError('Nama pemesan wajib diisi.')
    if (!address.trim()) return setError('Alamat wajib diisi.')
    if (!date) return setError('Tanggal wajib diisi.')
    if (overQuota) return setQuotaBlocked(true)

    // Hanya kirim yang berubah, supaya pengiriman yang jadwalnya sudah lewat tetap bisa diubah datanya.
    const patch: DeliveryPatch = {}
    if (name.trim() !== delivery.customer_name) patch.customer_name = name.trim()
    if (phone.trim() !== (delivery.customer_phone ?? '')) patch.customer_phone = phone.trim()
    if (address.trim() !== delivery.address) patch.address = address.trim()
    if (failed || date !== delivery.scheduled_date) patch.scheduled_date = date
    if (time !== (delivery.scheduled_time ?? '')) patch.scheduled_time = time
    if (courier.trim() !== (delivery.courier ?? '')) patch.courier = courier.trim()
    if (!fromPreorder && Number(fee || '0') !== delivery.fee) patch.fee = Number(fee || '0')
    if (notes.trim() !== (delivery.notes ?? '')) patch.notes = notes.trim()

    if (Object.keys(patch).length === 0) return onClose()

    setBusy(true)
    setError('')
    try {
      await updateDelivery(delivery.id, { ...patch, version })
      onSaved()
    } catch (err) {
      setStale(isStale(err))
      setError(isStale(err) ? staleMessage('Pengiriman') : message(err))
      setBusy(false)
    }
  }

  return (
    <Modal title={failed ? 'Jadwalkan ulang pengiriman' : 'Ubah pengiriman'} onClose={close} busy={busy}>
      {failed && (
        <p className="mt-2 text-ink-soft">Pilih tanggal baru. Setelah disimpan, statusnya kembali menjadi Menunggu.</p>
      )}
      {fromPreorder && (
        <p className="mt-2 text-ink-soft">
          Pemesan, alamat, jadwal, dan catatan sama dengan pre-order{' '}
          <span className="font-mono font-semibold text-ink">{delivery.source_no}</span>, jadi ikut berubah di sana.
        </p>
      )}
      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="mt-4 space-y-4">
        <div className="grid gap-x-3 gap-y-4 sm:gap-y-3 sm:grid-cols-2">
          <div>
            <label htmlFor="de-name" className={labelClass}>
              Nama pemesan
            </label>
            <input
              id="de-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={100}
              className={inputClass}
            />
          </div>
          <div>
            <label htmlFor="de-phone" className={labelClass}>
              No. HP
            </label>
            <input
              id="de-phone"
              type="tel"
              inputMode="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              maxLength={30}
              className={`${inputClass} font-mono`}
            />
          </div>
        </div>
        <div>
          <label htmlFor="de-address" className={labelClass}>
            Alamat
          </label>
          <textarea
            id="de-address"
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            maxLength={300}
            rows={2}
            className="w-full px-3 py-2 bg-white border-2 border-ink rounded-sm"
          />
        </div>
        <div className="grid gap-x-3 gap-y-4 sm:gap-y-3 sm:grid-cols-2">
          <div>
            <label htmlFor="de-date" className={labelClass}>
              Tanggal antar
            </label>
            <input
              id="de-date"
              type="date"
              min={toDateInput(new Date())}
              value={date}
              onChange={(e) => setDate(e.target.value)}
              autoFocus={failed}
              aria-describedby={moving ? 'de-date-quota' : undefined}
              className={inputClass}
            />
            {moving && <QuotaRemaining quota={quota} id="de-date-quota" />}
            {overQuota && quota && quota.remaining > 0 && (
              <p className="mt-1 text-sm font-semibold text-merah">{overQuota}</p>
            )}
          </div>
          <div>
            <label htmlFor="de-time" className={labelClass}>
              Jam
            </label>
            <input
              id="de-time"
              type="time"
              value={time}
              onChange={(e) => setTime(e.target.value)}
              className={inputClass}
            />
          </div>
          <div>
            <label htmlFor="de-courier" className={labelClass}>
              Kurir
            </label>
            <input
              id="de-courier"
              value={courier}
              onChange={(e) => setCourier(e.target.value)}
              maxLength={100}
              className={inputClass}
            />
          </div>
          <div>
            <label htmlFor="de-fee" className={labelClass}>
              Ongkir (Rp)
            </label>
            <input
              id="de-fee"
              inputMode="numeric"
              autoComplete="off"
              value={fee ? formatNumber(Number(fee)) : ''}
              onChange={(e) => setFee(digitsOnly(e.target.value))}
              disabled={fromPreorder}
              aria-describedby={fromPreorder ? 'de-fee-note' : undefined}
              className={`${inputClass} font-mono disabled:opacity-50`}
            />
            {fromPreorder && (
              <p id="de-fee-note" className="mt-1 text-sm text-ink-soft">
                Ongkir pre-order ikut tagihan pelanggan, jadi diubah dari detail pre-order.
              </p>
            )}
          </div>
        </div>
        <div>
          <label htmlFor="de-notes" className={labelClass}>
            Catatan
          </label>
          <input
            id="de-notes"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            maxLength={500}
            className={inputClass}
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
              {failed ? 'Jadwalkan ulang' : 'Simpan'}
            </button>
          )}
        </div>
      </form>
    </Modal>
  )
}

export function DispatchDialog({
  delivery,
  onClose,
  onSaved,
}: {
  delivery: Delivery
  onClose: () => void
  onSaved: () => void
}) {
  const [courier, setCourier] = useState(delivery.courier ?? '')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await dispatchDelivery(delivery.id, courier.trim())
      onSaved()
    } catch (err) {
      setError(message(err))
      setBusy(false)
    }
  }

  return (
    <Modal title="Berangkatkan pengiriman" onClose={onClose} busy={busy}>
      <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-4">
        <div>
          <label htmlFor="dp-courier" className={labelClass}>
            Nama kurir (opsional)
          </label>
          <input
            id="dp-courier"
            value={courier}
            onChange={(e) => setCourier(e.target.value)}
            autoFocus
            maxLength={100}
            autoComplete="off"
            className={inputClass}
          />
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
            Berangkat
          </button>
        </div>
      </form>
    </Modal>
  )
}
