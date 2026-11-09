import type { Transaction } from '../api/types'
import receiptLogo from '../assets/logo-fn-boss-struk.png'
import { STORE } from '../config'
import { formatRupiah } from '../lib/format'
import { PAYMENT_LABEL } from '../lib/payment'

const dateTime = new Intl.DateTimeFormat('id-ID', { dateStyle: 'short', timeStyle: 'short' })

// Isi struk. Dipakai untuk pratinjau di layar dan untuk dicetak.
export function ReceiptSheet({ transaction: t }: { transaction: Transaction }) {
  // Pre-order yang sudah lunas lewat DP: tidak ada pembayaran baru saat penyerahan.
  const settledByDp = t.down_payment > 0 && t.total - t.down_payment <= 0

  return (
    <div className="receipt">
      {t.status === 'void' && <div className="receipt-center receipt-strong">*** DIBATALKAN ***</div>}
      {/* Logo hitam-putih khusus printer thermal (merah jadi arsiran). Nama, no. HP, dan alamat sudah ada di logo. */}
      <img
        className="receipt-logo"
        src={receiptLogo}
        width={320}
        height={193}
        alt={`${STORE.name}, ${STORE.address}, ${STORE.phone}`}
      />
      <div className="receipt-rule" />
      <div>{dateTime.format(new Date(t.created_at))}</div>
      <div>No: {t.receipt_no}</div>
      <div>Kasir: {t.cashier_name}</div>
      <div className="receipt-rule" />

      {(t.items ?? []).map((item) => (
        <div key={item.id} className="receipt-item">
          <div>{item.product_name}</div>
          <div className="receipt-row">
            <span>
              {item.qty} x {formatRupiah(item.unit_price)}
            </span>
            <span>{formatRupiah(item.line_total)}</span>
          </div>
        </div>
      ))}

      <div className="receipt-rule" />
      {/* Penyerahan pre-order dengan ongkir atau biaya lain: rincian sebelum total. */}
      {t.shipping_fee + t.other_fee > 0 && (
        <>
          <div className="receipt-row">
            <span>Subtotal</span>
            <span>{formatRupiah(t.subtotal)}</span>
          </div>
          {t.other_fee > 0 && (
            <div className="receipt-row">
              <span>Biaya lain{t.other_fee_note ? ` (${t.other_fee_note})` : ''}</span>
              <span>{formatRupiah(t.other_fee)}</span>
            </div>
          )}
          {t.shipping_fee > 0 && (
            <div className="receipt-row">
              <span>Ongkir</span>
              <span>{formatRupiah(t.shipping_fee)}</span>
            </div>
          )}
        </>
      )}
      <div className="receipt-row receipt-strong">
        <span>Total</span>
        <span>{formatRupiah(t.total)}</span>
      </div>
      {t.down_payment > 0 && (
        <>
          <div className="receipt-row">
            <span>DP</span>
            <span>{formatRupiah(t.down_payment)}</span>
          </div>
          <div className="receipt-row">
            <span>Sisa</span>
            <span>{formatRupiah(Math.max(t.total - t.down_payment, 0))}</span>
          </div>
        </>
      )}
      {settledByDp ? (
        <div className="receipt-row">
          <span>Lunas dengan DP</span>
        </div>
      ) : (
        <>
          <div className="receipt-row">
            <span>{PAYMENT_LABEL[t.payment_method]}</span>
            <span>{formatRupiah(t.paid)}</span>
          </div>
          {t.payment_method === 'cash' && (
            <div className="receipt-row">
              <span>Kembali</span>
              <span>{formatRupiah(t.change)}</span>
            </div>
          )}
        </>
      )}
      <div className="receipt-rule" />
      <div className="receipt-center">{STORE.footer}</div>
    </div>
  )
}
