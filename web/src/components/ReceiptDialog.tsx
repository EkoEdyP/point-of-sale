import type { Transaction } from '../api/types'
import { formatRupiah } from '../lib/format'
import { Modal } from './Modal'
import { PrintArea, ReceiptPreview } from './ReceiptPreview'

// Muncul setelah pembayaran berhasil (Kasir dan pelunasan pre-order). Keranjang sudah dikosongkan saat bayar,
// jadi Tutup langsung kembali ke kasir yang siap untuk transaksi berikutnya.
export function ReceiptDialog({ transaction, onClose }: { transaction: Transaction; onClose: () => void }) {
  return (
    <>
      <Modal title="Pembayaran berhasil" onClose={onClose}>
        <p className="mt-1 font-mono text-xl font-semibold text-hijau">
          {transaction.payment_method === 'cash'
            ? `Kembalian ${formatRupiah(transaction.change)}`
            : 'Pembayaran diterima'}
        </p>

        <ReceiptPreview transaction={transaction} />

        <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:flex-wrap sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            className="h-12 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer"
          >
            Tutup
          </button>
          <button
            type="button"
            onClick={() => window.print()}
            className="h-12 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer"
          >
            Cetak struk
          </button>
        </div>
      </Modal>

      <PrintArea transaction={transaction} />
    </>
  )
}
