import { type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import type { Transaction } from '../api/types'
import { ReceiptSheet } from './ReceiptSheet'

// Pratinjau struk. Lebar kertasnya tetap 57 mm (printer kasir), lihat .receipt di index.css.
// scroll: struk panjang digulir di kotaknya sendiri, supaya tombol di bawahnya tetap terlihat (Pembayaran berhasil).
// scroll={false}: struk tampil utuh dan dialognya yang digulir (detail di Riwayat).
export function ReceiptPreview({ transaction, scroll = true }: { transaction: Transaction; scroll?: boolean }) {
  return (
    <div
      className={`mt-4 border border-line bg-white flex justify-center py-3 ${scroll ? 'max-h-[40vh] overflow-y-auto' : ''}`}
    >
      <ReceiptSheet transaction={transaction} />
    </div>
  )
}

// Satu-satunya yang tampil saat mencetak (lihat index.css). Isinya bebas: struk, surat jalan, dan lainnya.
export function PrintPortal({ children }: { children: ReactNode }) {
  return createPortal(<div id="print-area">{children}</div>, document.body)
}

export function PrintArea({ transaction }: { transaction: Transaction }) {
  return (
    <PrintPortal>
      <ReceiptSheet transaction={transaction} />
    </PrintPortal>
  )
}
