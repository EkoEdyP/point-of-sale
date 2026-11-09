import type { AuditEntity, AuditEntry, DeliveryStatus, PaymentMethod, PreorderStatus, Role } from '../api/types'
import { DELIVERY_STATUS_LABEL } from './delivery'
import { formatNumber, formatRupiah, formatShortDay } from './format'
import { PAYMENT_LABEL } from './payment'
import { STATUS_LABEL } from './preorder'
import { ROLE_LABEL } from './user'

// Hari riwayat perubahan mulai dicatat (tabel audit_logs dibuat). Perubahan sebelumnya tidak punya riwayat, kecuali
// pembatalan pre-order dan pengirimannya: pelaku, waktu, dan alasannya diisi belakangan dari data pre-order.
export const AUDIT_START = '2026-11-03'

// Pengaturan pre-order hanya satu baris (id 1); riwayat kuota harian dicatat dengan entity_id ini.
export const QUOTA_SETTING_ID = '1'

type Format =
  | 'text'
  | 'rupiah'
  | 'number'
  | 'pcs'
  | 'date'
  | 'time'
  | 'kind'
  | 'role'
  | 'preorderStatus'
  | 'deliveryStatus'
  | 'transactionStatus'
  | 'paymentMethod'

interface Field {
  label: string
  format: Format
}

// Kolom yang ditampilkan per jenis data, dalam urutan tampil. active, password, dan deleted_at punya kalimat
// sendiri (lihat describeAudit).
const FIELDS: Record<AuditEntity, Record<string, Field>> = {
  product: {
    kind: { label: 'Jenis', format: 'kind' },
    name: { label: 'Nama', format: 'text' },
    sku: { label: 'SKU', format: 'text' },
    price: { label: 'Harga jual', format: 'rupiah' },
    wholesale_price: { label: 'Harga grosir', format: 'rupiah' },
    wholesale_min_qty: { label: 'Grosir mulai dari', format: 'pcs' },
    stock: { label: 'Stok', format: 'number' },
  },
  preorder: {
    status: { label: 'Status', format: 'preorderStatus' },
    customer_name: { label: 'Pelanggan', format: 'text' },
    customer_phone: { label: 'No. HP', format: 'text' },
    address: { label: 'Alamat', format: 'text' },
    due_date: { label: 'Hari H', format: 'date' },
    due_time: { label: 'Jam', format: 'time' },
    notes: { label: 'Catatan', format: 'text' },
    shipping_fee: { label: 'Ongkir', format: 'rupiah' },
    other_fee: { label: 'Biaya lain', format: 'rupiah' },
    other_fee_note: { label: 'Keterangan biaya lain', format: 'text' },
    total: { label: 'Total', format: 'rupiah' },
  },
  delivery: {
    status: { label: 'Status', format: 'deliveryStatus' },
    status_note: { label: 'Keterangan', format: 'text' },
    customer_name: { label: 'Pemesan', format: 'text' },
    customer_phone: { label: 'No. telp', format: 'text' },
    address: { label: 'Alamat', format: 'text' },
    scheduled_date: { label: 'Tanggal kirim', format: 'date' },
    scheduled_time: { label: 'Jam kirim', format: 'time' },
    courier: { label: 'Kurir', format: 'text' },
    fee: { label: 'Ongkir', format: 'rupiah' },
    notes: { label: 'Catatan', format: 'text' },
    received_by: { label: 'Penerima', format: 'text' },
  },
  user: {
    name: { label: 'Nama', format: 'text' },
    username: { label: 'Username', format: 'text' },
    role: { label: 'Peran', format: 'role' },
  },
  transaction: {
    status: { label: 'Status', format: 'transactionStatus' },
  },
  setting: {
    daily_limit: { label: 'Kuota harian', format: 'pcs' },
  },
}

// Isi riwayat DP pre-order (aksi payment): jumlah dan metode saat diterima, lalu sisa tagihan sebelum dan sesudahnya.
const PAYMENT_FIELDS: Record<string, Field> = {
  amount: { label: 'Jumlah', format: 'rupiah' },
  method: { label: 'Metode', format: 'paymentMethod' },
  remaining: { label: 'Sisa tagihan', format: 'rupiah' },
}

const TRANSACTION_STATUS: Record<string, string> = { completed: 'Selesai', void: 'Dibatalkan' }

function isEmpty(value: unknown): boolean {
  return value === null || value === undefined || value === ''
}

function formatValue(format: Format, value: unknown): string {
  if (isEmpty(value)) return format === 'time' ? 'tanpa jam' : '(kosong)'
  const text = String(value)
  switch (format) {
    case 'rupiah':
      return formatRupiah(Number(value))
    case 'number':
      return formatNumber(Number(value))
    case 'pcs':
      return `${formatNumber(Number(value))} pcs`
    case 'date':
      return formatShortDay(text.slice(0, 10))
    case 'time':
      return text.slice(0, 5).replace(':', '.')
    case 'kind':
      return text === 'preorder' ? 'PO' : 'Toko'
    case 'role':
      return ROLE_LABEL[text as Role] ?? text
    case 'preorderStatus':
      return STATUS_LABEL[text as PreorderStatus] ?? text
    case 'deliveryStatus':
      return DELIVERY_STATUS_LABEL[text as DeliveryStatus] ?? text
    case 'transactionStatus':
      return TRANSACTION_STATUS[text] ?? text
    case 'paymentMethod':
      return PAYMENT_LABEL[text as PaymentMethod] ?? text
    default:
      return text
  }
}

interface ActionInfo {
  title: string
  danger?: boolean
  // Kolom yang sudah terwakili judulnya, misalnya status pada "Ditandai siap".
  hides?: string[]
  // Catatannya berisi alasan dari pengguna.
  reason?: boolean
}

const ACTIONS: Record<string, ActionInfo> = {
  delete: { title: 'Dihapus', danger: true, hides: ['active'] },
  ready: { title: 'Ditandai siap', hides: ['status'] },
  payment: { title: 'DP diterima' },
  complete: { title: 'Diserahkan dan selesai', hides: ['status'] },
  dispatch: { title: 'Berangkat', hides: ['status', 'status_note'] },
  deliver: { title: 'Terkirim', hides: ['status', 'status_note'] },
  fail: { title: 'Gagal dikirim', danger: true, hides: ['status', 'status_note'], reason: true },
  cancel: { title: 'Dibatalkan', danger: true, hides: ['status', 'status_note'], reason: true },
  void: { title: 'Dibatalkan', danger: true, hides: ['status'], reason: true },
}

// Catatan pengiriman yang ikut berubah bersama pre-order-nya. Ini keterangan dari server, bukan alasan pengguna.
const FOLLOWS_PREORDER = 'Ikut pre-order '

export type ChangeLine =
  | { kind: 'diff'; label: string; before: string; after: string } // nilai lama -> baru
  | { kind: 'value'; label: string; value: string } // nilai saat data dibuat
  | { kind: 'text'; text: string } // kalimat, misalnya "Password diganti"

export interface AuditDescription {
  title: string | null // ringkasan aksi; null untuk perubahan biasa yang cukup dengan daftar kolomnya
  danger: boolean
  lines: ChangeLine[]
  note: string | null
}

// Satu baris riwayat menjadi teks yang terbaca: judul aksi, kolom yang berubah, dan catatannya.
export function describeAudit(entry: AuditEntry): AuditDescription {
  const fields = FIELDS[entry.entity] ?? {}
  const changes = entry.changes ?? {}
  const action: ActionInfo | undefined = ACTIONS[entry.action]
  const hidden = new Set(action?.hides ?? [])
  const lines: ChangeLine[] = []
  let title = action?.title ?? null

  if (entry.action === 'create') {
    title = entry.entity === 'product' ? 'Produk baru' : entry.entity === 'user' ? 'Akun baru' : 'Dibuat'
    for (const [col, field] of Object.entries(fields)) {
      const value = changes[col]?.[1]
      // Nama produk dan username sudah tampil sebagai nama datanya.
      if (isEmpty(value) || (entry.entity === 'product' && col === 'name') || col === 'username') continue
      // Batas grosir hanya berarti kalau harga grosirnya diisi.
      if (col === 'wholesale_min_qty' && isEmpty(changes.wholesale_price?.[1])) continue
      lines.push({ kind: 'value', label: field.label, value: formatValue(field.format, value) })
    }
    if (changes.active?.[1] === false) lines.push({ kind: 'text', text: 'Nonaktif' })
  } else if (entry.action === 'payment') {
    for (const [col, field] of Object.entries(PAYMENT_FIELDS)) {
      if (!(col in changes)) continue
      const [before, after] = changes[col]
      lines.push(
        isEmpty(before)
          ? { kind: 'value', label: field.label, value: formatValue(field.format, after) }
          : {
              kind: 'diff',
              label: field.label,
              before: formatValue(field.format, before),
              after: formatValue(field.format, after),
            },
      )
    }
  } else {
    if (entry.action === 'production') {
      const [before, after] = changes.stock ?? [null, null]
      const added = Number(after) - Number(before)
      title = Number.isFinite(added) ? `Hasil produksi +${formatNumber(added)}` : 'Hasil produksi'
    }
    for (const [col, field] of Object.entries(fields)) {
      if (!(col in changes) || hidden.has(col)) continue
      const [before, after] = changes[col]
      // Keterangan status (alasan gagal atau batal) ikut terhapus saat statusnya berganti, misalnya dijadwalkan
      // ulang; baris statusnya sudah cukup.
      if (col === 'status_note' && 'status' in changes && isEmpty(after)) continue
      // Nama penerima baru ada saat terkirim: cukup namanya, bukan "(kosong) -> nama".
      if (col === 'received_by' && isEmpty(before)) {
        lines.push({ kind: 'value', label: field.label, value: formatValue(field.format, after) })
        continue
      }
      lines.push({
        kind: 'diff',
        label: entry.action === 'production' && col === 'stock' ? 'Stok siap' : field.label,
        before: formatValue(field.format, before),
        after: formatValue(field.format, after),
      })
    }
    if ('active' in changes && !hidden.has('active')) {
      lines.push({ kind: 'text', text: changes.active[1] ? 'Diaktifkan' : 'Dinonaktifkan' })
    }
    if ('password' in changes) lines.push({ kind: 'text', text: 'Password diganti' })
  }

  let note = entry.note
  if (note && action?.reason && !(entry.entity === 'delivery' && note.startsWith(FOLLOWS_PREORDER))) {
    note = `Alasan: ${note}`
  }
  if (!title && lines.length === 0) title = 'Diubah'
  return { title, danger: action?.danger ?? false, lines, note }
}
