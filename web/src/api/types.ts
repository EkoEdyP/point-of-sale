export type Role = 'admin' | 'cashier'

export interface User {
  id: string
  name: string
  username: string
  role: Role
  active: boolean
}

export interface AuthResponse {
  access_token: string
  token_type: string
  expires_in: number
  user: User
}

// Produk toko dijual di kasir; produk PO hanya bisa dipesan lewat pre-order.
export type ProductKind = 'store' | 'preorder'

export interface Product {
  id: string
  kind: ProductKind
  sku: string | null
  name: string
  category_id: string | null
  price: number
  // Harga grosir: null kalau tidak ada. Dipakai otomatis kalau beli wholesale_min_qty pcs atau lebih dalam satu
  // transaksi atau pre-order (lib/wholesale.ts).
  wholesale_price: number | null
  wholesale_min_qty: number
  stock: number // produk PO: stok siap (sudah diproduksi, belum diserahkan)
  active: boolean
  created_at: string
  updated_at: string
}

export interface ProductInput {
  kind: ProductKind // hanya dipakai saat membuat produk; jenis tidak bisa diubah
  name: string
  sku: string | null
  category_id: string | null
  price: number
  wholesale_price: number | null
  wholesale_min_qty: number | null // null = 50
  // Saat create: stok awal. Saat update server mengubah stok sebesar stock - stock_before (stok yang tampil saat
  // form dibuka), jadi penjualan selama form terbuka tidak tertimpa. stock_before null = stok tidak diubah.
  stock: number
  stock_before: number | null
  active: boolean
}

export interface ProductList {
  data: Product[]
  page: number
  limit: number
  total: number
}

export type PaymentMethod = 'cash' | 'qris' | 'ewallet' | 'card'

export interface TransactionItem {
  id: string
  product_id: string
  product_name: string
  unit_price: number
  qty: number
  discount: number
  line_total: number
}

// Asal transaksi: penjualan di kasir (toko) atau penyerahan pre-order.
export type TransactionSource = 'store' | 'preorder'

export interface Transaction {
  id: string
  receipt_no: string
  cashier_id: string
  cashier_name: string
  subtotal: number
  discount: number
  shipping_fee: number // ongkir pre-order (penjualan toko 0)
  other_fee: number // biaya lain pre-order, mis. kresek
  other_fee_note: string | null
  total: number // subtotal - diskon + ongkir + biaya lain
  paid: number
  change: number
  down_payment: number
  payment_method: PaymentMethod
  status: 'completed' | 'void'
  voided_by: string | null
  voided_by_name: string | null // akun yang membatalkan
  voided_at: string | null
  void_reason: string | null
  created_at: string
  source: TransactionSource
  preorder_id: string | null // terisi kalau source = preorder
  preorder_no: string | null
  items?: TransactionItem[]
}

export interface TransactionInput {
  items: { product_id: string; qty: number }[]
  payment_method: PaymentMethod
  paid: number
  // Total yang dilihat kasir. Kalau harga sudah berubah, server menolak dengan code "price_changed".
  expected_total?: number
}

export interface TransactionList {
  data: Transaction[]
  page: number
  limit: number
  total: number
}

export interface PaymentBreakdown {
  cash: number
  qris: number
  ewallet: number
  card: number
}

export interface ReportSummary {
  transaction_count: number
  items_sold: number
  revenue: number
  average_transaction: number
  by_payment_method: PaymentBreakdown
  dp_received: number
  dp_by_payment_method: PaymentBreakdown
  dp_cancelled: number
  dp_cancelled_by_payment_method: PaymentBreakdown
  void_count: number
  void_total: number
}

export interface DailyReportRow extends ReportSummary {
  date: string // YYYY-MM-DD
}

// Laporan dipisah: store (penjualan di kasir), preorder (penyerahan PO beserta DP), all (gabungan).
export type ReportSource = 'store' | 'preorder' | 'all'

export interface DailyReport {
  from: string
  to: string
  source: ReportSource
  days: DailyReportRow[]
  totals: ReportSummary
}

export interface UserList {
  data: User[]
}

export interface UserCreateInput {
  name: string
  username: string
  password: string
  role: Role
}

export interface UserPatch {
  name?: string
  active?: boolean
  password?: string
}

export type PreorderStatus = 'ordered' | 'ready' | 'completed' | 'cancelled'
export type Fulfillment = 'pickup' | 'delivery'

export interface PreorderItem {
  id: string
  product_id: string
  product_name: string
  unit_price: number
  qty: number
  line_total: number
}

export interface PreorderPayment {
  id: string
  amount: number
  method: PaymentMethod
  created_by_name: string
  created_at: string
}

export interface Preorder {
  id: string
  order_no: string
  customer_name: string
  customer_phone: string | null
  due_date: string // YYYY-MM-DD
  due_time: string | null // HH:MM
  fulfillment: Fulfillment
  address: string | null
  notes: string | null
  status: PreorderStatus
  subtotal: number // harga barang
  shipping_fee: number // ongkir (hanya pesanan diantar)
  other_fee: number // biaya lain, mis. kresek
  other_fee_note: string | null
  total: number // subtotal + ongkir + biaya lain
  down_payment: number
  remaining: number
  created_by_id: string
  created_by_name: string
  created_at: string
  transaction_id: string | null
  cancelled_at: string | null
  cancel_reason: string | null
  // Pengiriman pre-order diantar (null kalau diambil). Selama masih berjalan (lihat isActiveDelivery),
  // menyelesaikan pre-order ini ikut menandainya terkirim, dan nama penerimanya wajib diisi.
  delivery_no: string | null
  delivery_status: DeliveryStatus | null
  received_by: string | null
  version: number // naik setiap kali pre-order berubah; dikirim kembali saat mengubahnya
  items?: PreorderItem[]
  payments?: PreorderPayment[]
}

export interface PreorderList {
  data: Preorder[]
  page: number
  limit: number
  total: number
}

export interface PreorderSummary {
  overdue: number
  today: number
  tomorrow: number
  day_after_tomorrow: number // H-2: peringatan pre-order dimulai lusa
}

// Kebutuhan produksi dari pre-order yang belum diserahkan.
// Stok siap dipakai untuk hari H paling awal lebih dulu.
export interface ProductionItem {
  product_id: string
  product_name: string
  needed: number // butuh
  ready: number // sudah siap
  short: number // kurang
  orders: number // dari berapa pre-order
}

export interface ProductionDay {
  due_date: string
  items: ProductionItem[]
  quota_used: number // pcs yang memakai kuota hari H ini, termasuk pesanan yang sudah diserahkan
}

export interface ProductionProduct {
  product_id: string
  product_name: string
  stock: number // stok siap saat ini
  needed: number // untuk semua hari H
  short: number
}

export interface PreorderProduction {
  days: ProductionDay[]
  products: ProductionProduct[] // tidak ikut filter tanggal
  daily_limit: number // kuota pre-order per hari
}

// Kuota pre-order satu hari H: total pcs semua produk PO. Pesanan yang dibatalkan tidak dihitung,
// yang sudah diserahkan tetap dihitung.
export interface PreorderQuota {
  due_date: string
  daily_limit: number
  used: number
  remaining: number // tidak pernah negatif
}

export interface PreorderInput {
  customer_name: string
  customer_phone: string | null
  due_date: string
  due_time: string | null
  fulfillment: Fulfillment
  address: string | null
  notes: string | null
  items: { product_id: string; qty: number }[]
  shipping_fee: number
  other_fee: number
  other_fee_note: string | null
  down_payment: number
  down_payment_method: PaymentMethod | ''
  // Total yang tampil di form. Kalau harga sudah berubah, server menolak dengan code "price_changed".
  expected_total?: number
}

export type DeliveryStatus = 'pending' | 'on_the_way' | 'delivered' | 'failed' | 'cancelled'

export interface DeliveryItem {
  product_name: string
  qty: number
  unit_price: number // harga yang dikunci di pre-order
  line_total: number
}

export interface Delivery {
  id: string
  delivery_no: string
  source_type: 'preorder' | 'transaction'
  source_id: string
  source_no: string
  // Pemesan, alamat, jadwal, dan catatan pengiriman pre-order selalu sama dengan pre-order-nya.
  customer_name: string
  customer_phone: string | null
  address: string
  scheduled_date: string // YYYY-MM-DD
  scheduled_time: string | null // HH:MM
  courier: string | null
  fee: number
  notes: string | null
  status: DeliveryStatus
  status_note: string | null
  dispatched_at: string | null
  delivered_at: string | null
  received_by: string | null // nama penerima, diisi saat ditandai terkirim
  amount_due: number
  // Rincian uang untuk surat jalan: total = subtotal + ongkir (fee) + biaya lain; bayar ke kurir = amount_due.
  subtotal: number
  other_fee: number
  other_fee_note: string | null
  total: number
  down_payment: number
  // Status pre-order sumber (null untuk struk). Selama masih ordered/ready, Terkirim berarti menyelesaikan
  // pre-order-nya (completePreorder), yang sekaligus menandai pengiriman ini terkirim.
  preorder_status: PreorderStatus | null
  created_by_id: string
  created_by_name: string
  created_at: string
  version: number // seperti Preorder.version
  items?: DeliveryItem[]
}

export interface DeliveryList {
  data: Delivery[]
  page: number
  limit: number
  total: number
}

export interface DeliverySummary {
  overdue: number
  today: number
  tomorrow: number
}

export interface DeliveryPatch {
  customer_name?: string
  customer_phone?: string
  address?: string
  scheduled_date?: string
  scheduled_time?: string
  courier?: string
  fee?: number
  notes?: string
  version?: number // versi saat dialog ubah dibuka
}

// Jenis data yang dicatat riwayat perubahannya (audit_logs.entity).
export type AuditEntity = 'product' | 'preorder' | 'delivery' | 'user' | 'transaction' | 'setting'

// Satu perubahan data. changes = {"kolom": [lama, baru]}; kolom password hanya berisi "diubah". entity_label = nama
// atau nomor data saat perubahan terjadi. actor_name null untuk perubahan tanpa pengguna (skrip).
export interface AuditEntry {
  id: number
  created_at: string
  actor_id: string | null
  actor_name: string | null
  action: string
  entity: AuditEntity
  entity_id: string
  entity_label: string
  changes: Record<string, [unknown, unknown]>
  note: string | null
}

export interface AuditList {
  data: AuditEntry[]
  page: number
  limit: number
  total: number
}
