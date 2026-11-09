import { useEffect, useState, type ReactNode } from 'react'
import { ApiError } from '../api/client'
import { cancelDelivery, getDelivery, markFailed } from '../api/deliveries'
import { completeKeyScope, completePreorder } from '../api/preorders'
import type { Delivery, Transaction } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { DELIVERY_STATUS_LABEL, isScheduled } from '../lib/delivery'
import { formatDateTime, formatLongDate, formatRupiah } from '../lib/format'
import { withRequestKey } from '../lib/requestKey'
import { NOTA_PAGE_CSS } from '../lib/nota'
import { TONE_CLASS, dueInfo } from '../lib/preorder'
import { AlertIcon, CancelIcon, EditIcon, HistoryIcon, PrintIcon } from './ActionIcons'
import { ActionMenu, type MenuEntry } from './ActionMenu'
import { AuditHistoryDialog } from './AuditHistoryDialog'
import { DeliveryEditDialog, DispatchDialog } from './DeliveryDialogs'
import { DeliverySlip } from './DeliverySlip'
import { Modal } from './Modal'
import { PaymentDialog } from './PaymentDialog'
import { ReasonDialog } from './ReasonDialog'
import { ReceiverField } from './ReceiverField'
import { ReceiptDialog } from './ReceiptDialog'
import { PrintPortal } from './ReceiptPreview'

const actionClass = 'h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer disabled:opacity-60'
const primaryClass = 'h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer disabled:opacity-60'

function Info({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-sm text-ink-soft">{label}</dt>
      <dd className="font-semibold">{children}</dd>
    </div>
  )
}

export function DeliveryDetailDialog({
  id,
  onClose,
  onChanged,
}: {
  id: string
  onClose: () => void
  onChanged: () => void
}) {
  const { state } = useAuth()
  const me = state.status === 'authenticated' ? state.user : null

  const [delivery, setDelivery] = useState<Delivery | null>(null)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  const [editing, setEditing] = useState(false)
  const [showingHistory, setShowingHistory] = useState(false)
  const [dispatching, setDispatching] = useState(false)
  const [confirmingDelivered, setConfirmingDelivered] = useState(false)
  const [receivedBy, setReceivedBy] = useState('')
  const [confirmError, setConfirmError] = useState('')
  const [paying, setPaying] = useState(false)
  const [receipt, setReceipt] = useState<Transaction | null>(null)
  const [failing, setFailing] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let cancelled = false
    getDelivery(id)
      .then((d) => {
        if (!cancelled) setDelivery(d)
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat pengiriman.')
      })
    return () => {
      cancelled = true
    }
  }, [id, reloadKey])

  function changed() {
    setReloadKey((k) => k + 1)
    onChanged()
  }

  // Terkirim = menyelesaikan pre-order sumbernya (stok PO berkurang, transaksi tercatat), dan server ikut menandai
  // pengiriman ini terkirim, jadi keduanya selalu selesai bersama. Karena itu Terkirim hanya ada selama pre-order-nya
  // masih berjalan.
  const preorderOpen =
    delivery?.source_type === 'preorder' &&
    (delivery.preorder_status === 'ordered' || delivery.preorder_status === 'ready')

  // Pre-order yang sudah lunas lewat DP: diselesaikan tanpa pembayaran, karena tidak ada uang yang dibawa kurir.
  async function handleDelivered() {
    if (!delivery) return
    if (!receivedBy.trim()) return setConfirmError('Nama penerima wajib diisi.')
    setBusy(true)
    setError('')
    try {
      const poId = delivery.source_id
      const input = { received_by: receivedBy.trim() }
      const transaction = await withRequestKey(completeKeyScope(poId), (key) => completePreorder(poId, input, key))
      setReceipt(transaction)
      setConfirmingDelivered(false)
      changed()
    } catch (e) {
      setConfirmingDelivered(false)
      setError(e instanceof ApiError ? e.message : 'Gagal mengubah status.')
    } finally {
      setBusy(false)
    }
  }

  const status = delivery?.status
  const scheduled = status ? isScheduled(status) : false
  const canAct = scheduled || status === 'failed'
  const canCancel = canAct && me !== null && (me.role === 'admin' || me.id === delivery?.created_by_id)
  const due = delivery ? dueInfo(delivery.scheduled_date) : null
  // Penerima diisi saat ditandai terkirim; pengiriman yang terkirim sebelum itu tidak punya catatannya.
  const receiverHint = status === 'delivered' ? 'Tidak tercatat' : status === 'cancelled' ? '-' : 'Diisi saat terkirim'

  // Aksi tambahan masuk menu "Lainnya", sama seperti detail pre-order; di baris bawah tinggal Tutup dan aksi utama
  // sesuai status. Aksi berbahaya di bagian bawah menu, dipisah garis.
  const moreActions: MenuEntry[] = []
  if (scheduled) {
    moreActions.push({ label: 'Ubah pengiriman', icon: <EditIcon />, onSelect: () => setEditing(true) })
  }
  if (canAct) {
    moreActions.push({ label: 'Cetak surat jalan', icon: <PrintIcon />, onSelect: () => window.print() })
  }
  // Riwayat perubahan hanya untuk admin, juga untuk pengiriman yang sudah selesai atau dibatalkan.
  if (delivery && me?.role === 'admin') {
    moreActions.push({ label: 'Riwayat perubahan', icon: <HistoryIcon />, onSelect: () => setShowingHistory(true) })
  }
  const dangerActions: MenuEntry[] = []
  if (scheduled) {
    dangerActions.push({ label: 'Tandai gagal', icon: <AlertIcon />, danger: true, onSelect: () => setFailing(true) })
  }
  if (canCancel) {
    dangerActions.push({
      label: 'Batalkan pengiriman',
      icon: <CancelIcon />,
      danger: true,
      onSelect: () => setCancelling(true),
    })
  }
  if (dangerActions.length > 0) moreActions.push('separator', ...dangerActions)

  return (
    <>
      <Modal
        title={delivery ? `Pengiriman ${delivery.delivery_no}` : 'Detail pengiriman'}
        onClose={onClose}
        size="wide"
        busy={busy}
      >
        {error && (
          <p role="alert" className="mt-3 font-semibold text-merah">
            {error}
          </p>
        )}
        {!delivery && !error && <p className="mt-3 text-ink-soft">Memuat…</p>}

        {delivery && due && (
          <div className="mt-4 space-y-5">
            {delivery.status === 'failed' && (
              <p className="p-3 border-2 border-merah text-merah font-semibold">
                Gagal diantar{delivery.status_note ? `: ${delivery.status_note}` : ''}. Jadwalkan ulang atau batalkan.
              </p>
            )}
            {delivery.status === 'cancelled' && (
              <p className="p-3 border-2 border-merah text-merah font-semibold">
                Dibatalkan{delivery.status_note ? `: ${delivery.status_note}` : ''}.
              </p>
            )}
            {delivery.status === 'delivered' && (
              <p className="p-3 border-2 border-hijau text-hijau font-semibold">
                Terkirim{delivery.delivered_at ? ` pada ${formatDateTime(delivery.delivered_at)}` : ''}
                {delivery.received_by ? `, diterima ${delivery.received_by}` : ''}.
              </p>
            )}
            {delivery.amount_due > 0 && canAct && (
              <p className="p-3 border-2 border-ink bg-kunyit/30 font-semibold">
                Tagihan yang dibawa kurir: <span className="font-mono">{formatRupiah(delivery.amount_due)}</span>.
                <span className="block font-normal text-sm">
                  Ini sisa tagihan pre-order {delivery.source_no}. Uangnya dicatat saat menekan Terkirim, dan
                  pre-order-nya ikut selesai.
                </span>
              </p>
            )}

            <dl className="grid sm:grid-cols-2 gap-x-6 gap-y-3">
              <Info label="Jadwal">
                {scheduled && due.tone !== 'normal' && (
                  <span className={`mr-2 px-2 py-0.5 rounded-sm text-sm ${TONE_CLASS[due.tone]}`}>{due.label}</span>
                )}
                {formatLongDate(delivery.scheduled_date)}
                {delivery.scheduled_time ? `, ${delivery.scheduled_time}` : ''}
              </Info>
              <Info label="Status">{DELIVERY_STATUS_LABEL[delivery.status]}</Info>
              <Info label="Pemesan">
                {delivery.customer_name}
                {delivery.customer_phone && (
                  <span className="ml-2 font-mono text-sm">{delivery.customer_phone}</span>
                )}
              </Info>
              <Info label="Penerima">
                {delivery.received_by ?? <span className="font-normal text-ink-soft">{receiverHint}</span>}
              </Info>
              <div className="sm:col-span-2">
                <Info label="Alamat">{delivery.address}</Info>
              </div>
              <Info label="Kurir">{delivery.courier ?? <span className="font-normal text-ink-soft">Belum diisi</span>}</Info>
              <Info label="Ongkir">
                <span className="font-mono">{formatRupiah(delivery.fee)}</span>
              </Info>
              <Info label={delivery.source_type === 'preorder' ? 'Dari pre-order' : 'Dari struk'}>
                <span className="font-mono">{delivery.source_no}</span>
              </Info>
              <Info label="Dicatat oleh">
                {delivery.created_by_name}
                <span className="ml-2 font-normal text-sm text-ink-soft">{formatDateTime(delivery.created_at)}</span>
              </Info>
              {delivery.notes && (
                <div className="sm:col-span-2">
                  <Info label="Catatan">
                    <span className="font-normal">{delivery.notes}</span>
                  </Info>
                </div>
              )}
            </dl>

            <div>
              <h3 className="font-bold">Barang yang diantar</h3>
              <ul className="mt-1 divide-y divide-dashed divide-line">
                {(delivery.items ?? []).map((item) => (
                  <li key={item.product_name} className="py-1.5 flex justify-between gap-3">
                    <span>{item.product_name}</span>
                    <span className="font-mono font-semibold">{item.qty}</span>
                  </li>
                ))}
              </ul>
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
                {delivery.status === 'pending' && (
                  <button
                    type="button"
                    onClick={() => setDispatching(true)}
                    className={`${actionClass} order-2 col-span-2 @xl:order-none`}
                  >
                    Berangkat
                  </button>
                )}
                {scheduled && preorderOpen && (
                  <button
                    type="button"
                    onClick={() => {
                      setReceivedBy('')
                      setConfirmError('')
                      if (delivery.amount_due > 0) setPaying(true)
                      else setConfirmingDelivered(true)
                    }}
                    className={`${primaryClass} order-1 col-span-2 @xl:order-none`}
                  >
                    Terkirim
                  </button>
                )}
                {delivery.status === 'failed' && (
                  <button
                    type="button"
                    onClick={() => setEditing(true)}
                    className={`${primaryClass} order-1 col-span-2 @xl:order-none`}
                  >
                    Jadwalkan ulang
                  </button>
                )}
              </div>
            </div>
          </div>
        )}
      </Modal>

      {/* Surat jalan tidak dipasang selama struk pelunasan tampil, supaya Cetak struk hanya mencetak struk. */}
      {delivery && canAct && !receipt && (
        <PrintPortal>
          {/* Surat jalan dicetak di kertas nota pre-order: continuous form 9½ × 5½ inci (LQ-310). */}
          <style>{NOTA_PAGE_CSS}</style>
          <DeliverySlip delivery={delivery} />
        </PrintPortal>
      )}

      {delivery && showingHistory && (
        <AuditHistoryDialog
          entity="delivery"
          entityId={delivery.id}
          title={`Riwayat ${delivery.delivery_no}`}
          created={{ at: delivery.created_at, by: delivery.created_by_name, label: 'Pengiriman dibuat' }}
          onClose={() => setShowingHistory(false)}
        />
      )}

      {delivery && editing && (
        <DeliveryEditDialog
          delivery={delivery}
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false)
            changed()
          }}
        />
      )}

      {delivery && dispatching && (
        <DispatchDialog
          delivery={delivery}
          onClose={() => setDispatching(false)}
          onSaved={() => {
            setDispatching(false)
            changed()
          }}
        />
      )}

      {delivery && paying && (
        <PaymentDialog
          total={delivery.amount_due}
          title="Tandai terkirim"
          totalLabel="Tagihan yang dibawa kurir"
          submitLabel="Terkirim dan lunas"
          note={`Pre-order ${delivery.source_no} ikut selesai: stok PO berkurang dan transaksinya tercatat.`}
          receiver={{ customerName: delivery.customer_name }}
          onClose={() => setPaying(false)}
          onPay={async (method, paid, receivedBy) => {
            const poId = delivery.source_id
            const input = { payment_method: method, paid, received_by: receivedBy }
            const transaction = await withRequestKey(completeKeyScope(poId), (key) =>
              completePreorder(poId, input, key),
            )
            setPaying(false)
            setReceipt(transaction)
            changed()
          }}
        />
      )}

      {delivery && confirmingDelivered && (
        <Modal title="Tandai terkirim?" onClose={() => setConfirmingDelivered(false)} busy={busy}>
          <p className="mt-2 text-ink-soft">
            Pre-order {delivery.source_no} sudah lunas lewat DP. Menandai terkirim juga menyelesaikan pre-order-nya:
            stok PO berkurang dan transaksinya tercatat. Tidak bisa diurungkan.
          </p>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              void handleDelivered()
            }}
            noValidate
            className="mt-4 space-y-4"
          >
            <ReceiverField
              value={receivedBy}
              onChange={setReceivedBy}
              customerName={delivery.customer_name}
              autoFocus
            />
            {confirmError && (
              <p role="alert" className="font-semibold text-merah">
                {confirmError}
              </p>
            )}
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <button type="button" onClick={() => setConfirmingDelivered(false)} className={actionClass}>
                Batal
              </button>
              <button
                type="submit"
                disabled={busy}
                className="h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer disabled:opacity-60"
              >
                Tandai terkirim
              </button>
            </div>
          </form>
        </Modal>
      )}

      {failing && (
        <ReasonDialog
          title="Pengiriman gagal?"
          description={<p>Catat kenapa barang tidak sampai, misalnya rumah kosong. Pengiriman bisa dijadwalkan ulang.</p>}
          confirmLabel="Tandai gagal"
          onClose={() => setFailing(false)}
          onConfirm={async (reason) => {
            await markFailed(id, reason)
            setFailing(false)
            changed()
          }}
        />
      )}

      {cancelling && (
        <ReasonDialog
          title="Batalkan pengiriman?"
          description={<p>Pengiriman ini dibatalkan dan tidak bisa diurungkan.</p>}
          confirmLabel="Batalkan pengiriman"
          onClose={() => setCancelling(false)}
          onConfirm={async (reason) => {
            await cancelDelivery(id, reason)
            setCancelling(false)
            changed()
          }}
        />
      )}

      {receipt && <ReceiptDialog transaction={receipt} onClose={() => setReceipt(null)} />}
    </>
  )
}
