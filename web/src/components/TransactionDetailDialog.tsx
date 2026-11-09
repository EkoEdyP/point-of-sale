import { useEffect, useState } from 'react'
import { ApiError } from '../api/client'
import { getPreorder } from '../api/preorders'
import { getTransaction } from '../api/transactions'
import type { Preorder, Transaction } from '../api/types'
import { formatDateTime } from '../lib/format'
import { CancelIcon, PrintIcon } from './ActionIcons'
import { ActionMenu, type MenuEntry } from './ActionMenu'
import { Modal } from './Modal'
import { PreorderNotaDialog } from './PreorderNota'
import { PrintArea, ReceiptPreview } from './ReceiptPreview'
import { SourceBadge } from './SourceBadge'
import { VoidDialog } from './VoidDialog'

const actionClass = 'h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer'

export function TransactionDetailDialog({
  id,
  onClose,
  onChanged,
}: {
  id: string
  onClose: () => void
  onChanged: () => void
}) {
  const [transaction, setTransaction] = useState<Transaction | null>(null)
  const [error, setError] = useState('')
  const [voiding, setVoiding] = useState(false)
  // Nota PO untuk penyerahan pre-order, dimuat dari pre-order-nya saat diminta.
  const [nota, setNota] = useState<Preorder | null>(null)
  const [loadingNota, setLoadingNota] = useState(false)
  const [notaError, setNotaError] = useState('')

  useEffect(() => {
    let cancelled = false
    getTransaction(id)
      .then((t) => {
        if (!cancelled) setTransaction(t)
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat transaksi.')
      })
    return () => {
      cancelled = true
    }
  }, [id])

  const openNota = (preorderId: string) => {
    setNotaError('')
    setLoadingNota(true)
    getPreorder(preorderId)
      .then(setNota)
      .catch((e) => setNotaError(e instanceof ApiError ? e.message : 'Gagal memuat nota pre-order.'))
      .finally(() => setLoadingNota(false))
  }

  // Aksi tambahan di menu "Lainnya"; di baris bawah tinggal Tutup dan Cetak ulang struk. Penyerahan pre-order bisa
  // mencetak ulang nota PO-nya, tapi tidak bisa dibatalkan (server menolaknya), karena PO-nya tetap selesai.
  const moreActions: MenuEntry[] = []
  const preorderId = transaction?.source === 'preorder' ? transaction.preorder_id : null
  if (preorderId) {
    moreActions.push({ label: 'Cetak ulang nota', icon: <PrintIcon />, onSelect: () => openNota(preorderId) })
  }
  if (transaction?.status === 'completed' && transaction.source !== 'preorder') {
    moreActions.push({
      label: 'Batalkan transaksi',
      icon: <CancelIcon />,
      danger: true,
      onSelect: () => setVoiding(true),
    })
  }

  return (
    <>
      <Modal title={transaction ? `Transaksi ${transaction.receipt_no}` : 'Detail transaksi'} onClose={onClose}>
        {error && (
          <p role="alert" className="mt-3 font-semibold text-merah">
            {error}
          </p>
        )}
        {!transaction && !error && <p className="mt-3 text-ink-soft">Memuat…</p>}

        {transaction && (
          <>
            <p className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
              <SourceBadge source={transaction.source} />
              <span className="text-ink-soft">
                {transaction.source === 'preorder'
                  ? `Penyerahan pre-order ${transaction.preorder_no ?? ''}`.trim()
                  : 'Penjualan di kasir'}
              </span>
            </p>

            {transaction.status === 'void' && (
              <p className="mt-3 p-3 border-2 border-merah text-merah font-semibold">
                Dibatalkan{transaction.voided_at ? ` pada ${formatDateTime(transaction.voided_at)}` : ''}
                {transaction.voided_by_name ? ` oleh ${transaction.voided_by_name}` : ''}.
                {transaction.void_reason ? ` Alasan: ${transaction.void_reason}` : ''}
              </p>
            )}

            <ReceiptPreview transaction={transaction} scroll={false} />

            {/* HP: Cetak ulang struk paling atas selebar dialog, lalu Lainnya | Tutup.
                Isi dialog >= 25rem (tombolnya butuh ±24rem): Lainnya di kiri, Tutup dan Cetak ulang di kanan. */}
            <div className="mt-5 @container">
              <div className="grid grid-cols-2 gap-2 *:whitespace-nowrap @min-[25rem]:flex @min-[25rem]:items-center">
                {moreActions.length > 0 && (
                  <ActionMenu entries={moreActions} className={`${actionClass} order-3 @min-[25rem]:order-none`} />
                )}
                <button
                  type="button"
                  onClick={onClose}
                  className={`${actionClass} order-4 @min-[25rem]:order-none @min-[25rem]:ml-auto ${
                    moreActions.length > 0 ? '' : 'col-span-2'
                  }`}
                >
                  Tutup
                </button>
                <button
                  type="button"
                  onClick={() => window.print()}
                  className="order-1 col-span-2 @min-[25rem]:order-none h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer"
                >
                  Cetak ulang struk
                </button>
              </div>
              {loadingNota && <p className="mt-3 text-sm text-ink-soft">Memuat nota…</p>}
              {notaError && (
                <p role="alert" className="mt-3 font-semibold text-merah">
                  {notaError}
                </p>
              )}
            </div>
          </>
        )}
      </Modal>

      {/* Hanya satu area cetak: selama nota terbuka, yang tercetak notanya, bukan struk. */}
      {transaction && !nota && <PrintArea transaction={transaction} />}
      {nota && <PreorderNotaDialog preorder={nota} onClose={() => setNota(null)} />}

      {voiding && transaction && (
        <VoidDialog
          transaction={transaction}
          onClose={() => setVoiding(false)}
          onVoided={(updated) => {
            setTransaction(updated)
            setVoiding(false)
            onChanged()
          }}
        />
      )}
    </>
  )
}
