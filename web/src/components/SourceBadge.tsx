import type { TransactionSource } from '../api/types'

// Label asal transaksi di Riwayat: Toko (penjualan di kasir) atau PO (penyerahan pre-order).
export function SourceBadge({ source }: { source: TransactionSource }) {
  return source === 'preorder' ? (
    <span className="px-1.5 py-0.5 text-xs font-semibold bg-ink border border-ink text-paper rounded-sm">PO</span>
  ) : (
    <span className="px-1.5 py-0.5 text-xs font-semibold border border-ink-soft text-ink-soft rounded-sm">Toko</span>
  )
}
