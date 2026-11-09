import type { CSSProperties } from 'react'
import { formatNumber } from './format'

// Kertas nota pre-order dan surat jalan: continuous form 9½ × 5½ inci untuk printer Epson LQ-310.
// Di layar 1 inci = 96 px.
export const NOTA_PAGE_CSS = '@page { size: 241.3mm 139.7mm; margin: 0; }'
export const NOTA_PAGE_W = 9.5 * 96

// Baris tabel minimal, seperti nota cetakan toko. Kalau barangnya lebih banyak, baris dipadatkan
// (paling rendah 4,6 mm) supaya tetap satu lembar; lebih dari itu kertasnya memanjang ke lembar berikutnya.
// Surat jalan memakai minimal 6 baris karena kepalanya punya baris catatan; nota pre-order yang punya catatan
// memakai minimal 5 baris.
const ROWS = 7
const ROW_MM = 6.8

export function notaRows(count: number, minRows = ROWS): { empty: number; style?: CSSProperties } {
  const rowMm = Math.max(4.6, (minRows * ROW_MM) / count)
  return {
    empty: Math.max(minRows - count, 0),
    style: count > minRows ? ({ '--nota-row': `${rowMm}mm` } as CSSProperties) : undefined,
  }
}

// Angka di rincian bayar; 0 ditulis "-" seperti nota tulisan tangan.
export function notaAmount(value: number) {
  return value > 0 ? formatNumber(value) : '-'
}
