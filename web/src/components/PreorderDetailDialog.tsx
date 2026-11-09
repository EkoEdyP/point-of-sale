import { useEffect, useState, type ReactNode } from 'react'
import { ApiError } from '../api/client'
import { cancelPreorder, completeKeyScope, completePreorder, getPreorder, markPreorderReady } from '../api/preorders'
import type { Preorder, Transaction } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { DELIVERY_STATUS_LABEL, isActiveDelivery } from '../lib/delivery'
import { formatDateTime, formatLongDate, formatRupiah } from '../lib/format'
import { withRequestKey } from '../lib/requestKey'
import { PAYMENT_LABEL } from '../lib/payment'
import { FULFILLMENT_LABEL, PREORDER_WARN_DAYS, STATUS_LABEL, TONE_CLASS, dueInfo } from '../lib/preorder'
import { CalendarIcon, CancelIcon, EditIcon, HistoryIcon, PlusIcon, PrintIcon, UserIcon } from './ActionIcons'
import { ActionMenu, type MenuEntry } from './ActionMenu'
import { AuditHistoryDialog } from './AuditHistoryDialog'
import { Modal } from './Modal'
import { PaymentDialog } from './PaymentDialog'
import { AddDownPaymentDialog, CustomerDialog, FeesDialog, ScheduleDialog } from './PreorderDialogs'
import { PreorderNotaDialog } from './PreorderNota'
import { ReasonDialog } from './ReasonDialog'
import { ReceiverField } from './ReceiverField'
import { ReceiptDialog } from './ReceiptDialog'

const actionClass = 'h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer disabled:opacity-60'

function Info({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-sm text-ink-soft">{label}</dt>
      <dd className="font-semibold">{children}</dd>
    </div>
  )
}

// Satu baris rincian bayar: label kiri, nominal rata kanan (turun ke baris sendiri kalau tidak muat).
function SumRow({ label, amount, className = '' }: { label: ReactNode; amount: number; className?: string }) {
  return (
    <div className={`flex flex-wrap items-baseline justify-between gap-x-3 ${className}`}>
      <dt className="min-w-0 break-words">{label}</dt>
      <dd className="ml-auto font-mono">{formatRupiah(amount)}</dd>
    </div>
  )
}

// Alasan diketik bebas oleh pengguna; diberi titik kalau belum, supaya kalimat sesudahnya tidak menempel.
const asSentence = (text: string) => (/[.!?]$/.test(text.trim()) ? text.trim() : `${text.trim()}.`)

export function PreorderDetailDialog({
  id,
  openNota = false,
  onClose,
  onChanged,
}: {
  id: string
  // true: nota langsung ditawarkan untuk dicetak (pre-order baru dibuat).
  openNota?: boolean
  onClose: () => void
  onChanged: () => void
}) {
  const { state } = useAuth()
  const me = state.status === 'authenticated' ? state.user : null

  const [preorder, setPreorder] = useState<Preorder | null>(null)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const [busy, setBusy] = useState(false)

  const [addingDp, setAddingDp] = useState(false)
  const [scheduling, setScheduling] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const [paying, setPaying] = useState(false)
  const [confirmingPaid, setConfirmingPaid] = useState(false)
  const [receipt, setReceipt] = useState<Transaction | null>(null)
  const [printingNota, setPrintingNota] = useState(openNota)
  const [showingHistory, setShowingHistory] = useState(false)
  const [editingFees, setEditingFees] = useState(false)
  const [editingCustomer, setEditingCustomer] = useState(false)
  const [receivedBy, setReceivedBy] = useState('')
  const [confirmError, setConfirmError] = useState('')

  useEffect(() => {
    let cancelled = false
    getPreorder(id)
      .then((p) => {
        if (!cancelled) setPreorder(p)
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat pre-order.')
      })
    return () => {
      cancelled = true
    }
  }, [id, reloadKey])

  // Pengiriman yang masih berjalan ikut terkirim saat pre-order ini diselesaikan, jadi penerimanya dicatat.
  const needsReceiver = isActiveDelivery(preorder?.delivery_status)

  function changed() {
    setReloadKey((k) => k + 1)
    onChanged()
  }

  async function handleReady() {
    setBusy(true)
    setError('')
    try {
      await markPreorderReady(id)
      changed()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Gagal mengubah status.')
    } finally {
      setBusy(false)
    }
  }

  async function finishWithoutPayment() {
    if (needsReceiver && !receivedBy.trim()) return setConfirmError('Nama penerima wajib diisi.')
    setBusy(true)
    setError('')
    try {
      const input = needsReceiver ? { received_by: receivedBy.trim() } : {}
      const transaction = await withRequestKey(completeKeyScope(id), (key) => completePreorder(id, input, key))
      setConfirmingPaid(false)
      setReceipt(transaction)
      changed()
    } catch (e) {
      setConfirmingPaid(false)
      setError(e instanceof ApiError ? e.message : 'Gagal menyelesaikan pre-order.')
    } finally {
      setBusy(false)
    }
  }

  const isOpen = preorder?.status === 'ordered' || preorder?.status === 'ready'
  const canCancel = isOpen && me !== null && (me.role === 'admin' || me.id === preorder?.created_by_id)
  const due = preorder ? dueInfo(preorder.due_date, PREORDER_WARN_DAYS) : null

  // Aksi tambahan masuk menu "Lainnya"; di baris bawah tinggal Tutup dan aksi utama.
  const moreActions: MenuEntry[] = []
  if (preorder && isOpen) {
    moreActions.push({ label: 'Cetak nota', icon: <PrintIcon />, onSelect: () => setPrintingNota(true) })
    if (preorder.remaining > 0) {
      moreActions.push({ label: 'Tambah DP', icon: <PlusIcon />, onSelect: () => setAddingDp(true) })
    }
    moreActions.push(
      { label: 'Ubah data pelanggan', icon: <UserIcon />, onSelect: () => setEditingCustomer(true) },
      { label: 'Ubah jadwal', icon: <CalendarIcon />, onSelect: () => setScheduling(true) },
      {
        label: preorder.fulfillment === 'delivery' ? 'Ubah ongkir dan biaya lain' : 'Ubah biaya lain',
        icon: <EditIcon />,
        onSelect: () => setEditingFees(true),
      },
    )
  }
  // Riwayat perubahan hanya untuk admin, juga untuk pre-order yang sudah selesai atau dibatalkan.
  if (preorder && me?.role === 'admin') {
    moreActions.push({ label: 'Riwayat perubahan', icon: <HistoryIcon />, onSelect: () => setShowingHistory(true) })
  }
  if (canCancel) {
    moreActions.push('separator', {
      label: 'Batalkan pre-order',
      icon: <CancelIcon />,
      danger: true,
      onSelect: () => setCancelling(true),
    })
  }

  return (
    <>
      <Modal
        title={preorder ? `Pre-order ${preorder.order_no}` : 'Detail pre-order'}
        onClose={onClose}
        size="wide"
        busy={busy}
      >
        {error && (
          <p role="alert" className="mt-3 font-semibold text-merah">
            {error}
          </p>
        )}
        {!preorder && !error && <p className="mt-3 text-ink-soft">Memuat…</p>}

        {preorder && due && (
          <div className="mt-4 space-y-5">
            {preorder.status === 'cancelled' && (
              <p className="p-3 border-2 border-merah text-merah font-semibold">
                Dibatalkan{preorder.cancelled_at ? ` pada ${formatDateTime(preorder.cancelled_at)}` : ''}.
                {preorder.cancel_reason ? ` Alasan: ${asSentence(preorder.cancel_reason)}` : ''}
                {preorder.down_payment > 0 && ' DP yang sudah diterima dikembalikan ke pelanggan secara manual.'}
              </p>
            )}
            {preorder.status === 'completed' && (
              <p className="p-3 border-2 border-hijau text-hijau font-semibold">
                Selesai. Transaksinya tercatat dan struknya bisa dicetak ulang dari menu Riwayat.
              </p>
            )}

            <dl className="grid sm:grid-cols-2 gap-x-6 gap-y-3">
              <Info label="Hari H">
                {isOpen && due.tone !== 'normal' && (
                  <span className={`mr-2 px-2 py-0.5 rounded-sm text-sm ${TONE_CLASS[due.tone]}`}>{due.label}</span>
                )}
                {formatLongDate(preorder.due_date)}
                {preorder.due_time ? `, ${preorder.due_time}` : ''}
              </Info>
              <Info label="Status">{STATUS_LABEL[preorder.status]}</Info>
              <Info label="Pelanggan">
                {preorder.customer_name}
                {preorder.customer_phone && <span className="ml-2 font-mono text-sm">{preorder.customer_phone}</span>}
              </Info>
              <Info label="Cara serah">
                {FULFILLMENT_LABEL[preorder.fulfillment]}
                {preorder.address && <span className="block font-normal">{preorder.address}</span>}
                {preorder.delivery_no && preorder.delivery_status && (
                  <span className="block text-sm font-normal text-ink-soft">
                    Pengiriman <span className="font-mono">{preorder.delivery_no}</span>:{' '}
                    {DELIVERY_STATUS_LABEL[preorder.delivery_status]}
                    {preorder.received_by ? `, diterima ${preorder.received_by}` : ''}.
                    {needsReceiver && (
                      <>
                        {' '}
                        Data pelanggan, jadwal, dan catatannya selalu sama dengan pre-order ini, dan ikut terkirim
                        saat pre-order ini diselesaikan.
                      </>
                    )}
                  </span>
                )}
              </Info>
              {preorder.notes && (
                <div className="sm:col-span-2">
                  <Info label="Catatan">
                    <span className="font-normal">{preorder.notes}</span>
                  </Info>
                </div>
              )}
              <Info label="Dicatat oleh">
                {preorder.created_by_name}
                <span className="ml-2 font-normal text-sm text-ink-soft">{formatDateTime(preorder.created_at)}</span>
              </Info>
            </dl>

            {/* HP: daftar ringkas, karena empat kolom tabel tidak muat di dialog selebar layar. */}
            <ul className="sm:hidden border border-line divide-y divide-line">
              {(preorder.items ?? []).map((item) => (
                <li key={item.id} className="px-3 py-2">
                  <div className="flex items-baseline justify-between gap-3">
                    <span className="min-w-0 font-semibold break-words">{item.product_name}</span>
                    <span className="shrink-0 font-mono font-semibold">{formatRupiah(item.line_total)}</span>
                  </div>
                  <div className="font-mono text-sm text-ink-soft">
                    {item.qty} × {formatRupiah(item.unit_price)}
                  </div>
                </li>
              ))}
            </ul>

            <div className="hidden sm:block overflow-x-auto border border-line">
              <table className="w-full text-left">
                <thead>
                  <tr className="border-b-2 border-ink text-sm text-ink-soft">
                    <th className="px-3 py-2 font-semibold">Barang</th>
                    <th className="px-3 py-2 font-semibold text-right">Harga</th>
                    <th className="px-3 py-2 font-semibold text-right">Jumlah</th>
                    <th className="px-3 py-2 font-semibold text-right">Subtotal</th>
                  </tr>
                </thead>
                <tbody>
                  {(preorder.items ?? []).map((item) => (
                    <tr key={item.id} className="border-b border-line">
                      <td className="px-3 py-2 font-semibold">{item.product_name}</td>
                      <td className="px-3 py-2 font-mono text-right">{formatRupiah(item.unit_price)}</td>
                      <td className="px-3 py-2 font-mono text-right">{item.qty}</td>
                      <td className="px-3 py-2 font-mono text-right">{formatRupiah(item.line_total)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Rincian bayar: label huruf biasa, nominal rata kanan. Satu DP = satu baris (cara bayar + nominal),
                waktu dan pencatatnya di bawahnya. */}
            <div className="grid sm:grid-cols-2 gap-x-8 gap-y-5">
              <div className="min-w-0">
                <h3 className="font-bold">DP diterima</h3>
                {(preorder.payments ?? []).length === 0 ? (
                  <p className="mt-1 text-ink-soft">Belum ada DP.</p>
                ) : (
                  <ul className="mt-1 divide-y divide-dashed divide-line">
                    {(preorder.payments ?? []).map((p) => (
                      <li key={p.id} className="py-2 flex items-baseline justify-between gap-3">
                        <div className="min-w-0">
                          <div className="font-semibold">{PAYMENT_LABEL[p.method]}</div>
                          <div className="text-sm text-ink-soft break-words">
                            {formatDateTime(p.created_at)} · {p.created_by_name}
                          </div>
                        </div>
                        <span className="shrink-0 font-mono font-semibold">{formatRupiah(p.amount)}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              <dl className="min-w-0 space-y-1">
                {preorder.shipping_fee + preorder.other_fee > 0 && (
                  <>
                    <SumRow label="Subtotal barang" amount={preorder.subtotal} />
                    {preorder.other_fee > 0 && (
                      <SumRow
                        label={
                          <>
                            Biaya lain
                            {preorder.other_fee_note && (
                              <span className="text-sm text-ink-soft"> ({preorder.other_fee_note})</span>
                            )}
                          </>
                        }
                        amount={preorder.other_fee}
                      />
                    )}
                    {preorder.shipping_fee > 0 && <SumRow label="Ongkir" amount={preorder.shipping_fee} />}
                  </>
                )}
                <SumRow label="Total" amount={preorder.total} className="font-bold" />
                <SumRow label="DP" amount={preorder.down_payment} />
                <SumRow
                  label="Sisa tagihan"
                  amount={preorder.remaining}
                  className="mt-2 pt-2 border-t-2 border-dashed border-ink text-lg font-bold"
                />
              </dl>
            </div>

            {/* HP: aksi utama paling atas selebar dialog, lalu Lainnya | Tutup.
                Dialog lebar (isi >= 36rem): Lainnya di kiri, Tutup dan aksi utama di kanan, satu baris. */}
            <div className="@container">
              <div className="grid grid-cols-2 gap-2 *:whitespace-nowrap @xl:flex @xl:items-center">
                {moreActions.length > 0 && (
                  <ActionMenu entries={moreActions} className={`${actionClass} order-3 @xl:order-none`} />
                )}
                <button
                  type="button"
                  onClick={onClose}
                  className={`${actionClass} order-4 @xl:order-none @xl:ml-auto ${
                    moreActions.length > 0 ? '' : 'col-span-2'
                  }`}
                >
                  Tutup
                </button>
                {preorder.status === 'ordered' && (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => void handleReady()}
                    className={`${actionClass} order-2 col-span-2 @xl:order-none`}
                  >
                    Tandai siap
                  </button>
                )}
                {isOpen && (
                  <button
                    type="button"
                    onClick={() => {
                      setReceivedBy('')
                      setConfirmError('')
                      if (preorder.remaining > 0) setPaying(true)
                      else setConfirmingPaid(true)
                    }}
                    className="order-1 col-span-2 @xl:order-none h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer"
                  >
                    Serahkan dan selesaikan
                  </button>
                )}
              </div>
            </div>
          </div>
        )}
      </Modal>

      {preorder && showingHistory && (
        <AuditHistoryDialog
          entity="preorder"
          entityId={preorder.id}
          title={`Riwayat ${preorder.order_no}`}
          created={{ at: preorder.created_at, by: preorder.created_by_name, label: 'Pre-order dibuat' }}
          onClose={() => setShowingHistory(false)}
        />
      )}

      {preorder && addingDp && (
        <AddDownPaymentDialog
          preorder={preorder}
          onClose={() => setAddingDp(false)}
          onSaved={() => {
            setAddingDp(false)
            changed()
          }}
        />
      )}

      {/* Nota hanya untuk pre-order berjalan; yang sudah selesai buktinya struk di Riwayat. */}
      {preorder && isOpen && printingNota && (
        <PreorderNotaDialog preorder={preorder} onClose={() => setPrintingNota(false)} />
      )}

      {preorder && isOpen && editingFees && (
        <FeesDialog
          preorder={preorder}
          onClose={() => setEditingFees(false)}
          onSaved={() => {
            setEditingFees(false)
            changed()
          }}
        />
      )}

      {preorder && isOpen && editingCustomer && (
        <CustomerDialog
          preorder={preorder}
          onClose={() => setEditingCustomer(false)}
          onSaved={() => {
            setEditingCustomer(false)
            changed()
          }}
        />
      )}

      {preorder && scheduling && (
        <ScheduleDialog
          preorder={preorder}
          onClose={() => setScheduling(false)}
          onSaved={() => {
            setScheduling(false)
            changed()
          }}
        />
      )}

      {preorder && cancelling && (
        <ReasonDialog
          title="Batalkan pre-order?"
          description={
            <p>
              Pre-order <span className="font-mono font-semibold text-ink">{preorder.order_no}</span> dibatalkan dan
              tidak bisa diurungkan.
              {preorder.down_payment > 0 &&
                ` DP ${formatRupiah(preorder.down_payment)} yang sudah diterima dikembalikan ke pelanggan secara manual.`}
            </p>
          }
          confirmLabel="Batalkan pre-order"
          onClose={() => setCancelling(false)}
          onConfirm={async (reason) => {
            await cancelPreorder(id, reason)
            setCancelling(false)
            changed()
          }}
        />
      )}

      {preorder && paying && (
        <PaymentDialog
          total={preorder.remaining}
          title="Pelunasan"
          totalLabel="Sisa pembayaran"
          submitLabel="Lunasi dan selesaikan"
          note={needsReceiver ? 'Pengirimannya ikut ditandai terkirim.' : undefined}
          receiver={needsReceiver ? { customerName: preorder.customer_name } : undefined}
          onClose={() => setPaying(false)}
          onPay={async (method, paid, receivedBy) => {
            const input = { payment_method: method, paid, received_by: needsReceiver ? receivedBy : undefined }
            const transaction = await withRequestKey(completeKeyScope(id), (key) => completePreorder(id, input, key))
            setPaying(false)
            setReceipt(transaction)
            changed()
          }}
        />
      )}

      {preorder && confirmingPaid && (
        <Modal title="Serahkan barang?" onClose={() => setConfirmingPaid(false)} busy={busy}>
          <p className="mt-2 text-ink-soft">
            Pre-order ini sudah lunas lewat DP. Menyelesaikannya mengurangi stok dan mencatat transaksi.
            {needsReceiver && ' Pengirimannya ikut ditandai terkirim.'}
          </p>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              void finishWithoutPayment()
            }}
            noValidate
            className="mt-4 space-y-4"
          >
            {needsReceiver && (
              <ReceiverField
                value={receivedBy}
                onChange={setReceivedBy}
                customerName={preorder.customer_name}
                autoFocus
              />
            )}
            {confirmError && (
              <p role="alert" className="font-semibold text-merah">
                {confirmError}
              </p>
            )}
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <button type="button" onClick={() => setConfirmingPaid(false)} className={actionClass}>
                Batal
              </button>
              <button
                type="submit"
                disabled={busy}
                className="h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer disabled:opacity-60"
              >
                Selesaikan pre-order
              </button>
            </div>
          </form>
        </Modal>
      )}

      {receipt && <ReceiptDialog transaction={receipt} onClose={() => setReceipt(null)} />}
    </>
  )
}
