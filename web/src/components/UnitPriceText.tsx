import { formatRupiah } from '../lib/format'
import { unitPrice, type PriceRule } from '../lib/wholesale'

// Harga satuan di baris keranjang Kasir dan form pre-order. Kalau harga grosir berlaku, harga normal dicoret
// dan diberi tanda "grosir". align: 'end' untuk kolom kanan (Kasir), 'start' di bawah nama barang (pre-order).
export function UnitPriceText({ rule, qty, align }: { rule: PriceRule; qty: number; align: 'start' | 'end' }) {
  const unit = unitPrice(rule, qty)
  if (unit === rule.price) return <p className="font-mono text-xs text-ink-soft">{formatRupiah(unit)}</p>

  return (
    <p
      className={`flex flex-wrap items-center gap-x-1.5 gap-y-0.5 font-mono text-xs ${align === 'end' ? 'justify-end' : ''}`}
    >
      <s className="text-ink-soft">
        <span className="sr-only">Harga normal </span>
        {formatRupiah(rule.price)}
      </s>
      <span className="font-semibold">{formatRupiah(unit)}</span>
      <span className="px-1.5 rounded-full bg-hijau font-sans text-[11px] font-bold leading-4 text-white">grosir</span>
    </p>
  )
}
