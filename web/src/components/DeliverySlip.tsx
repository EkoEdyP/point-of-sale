import type { Delivery } from '../api/types'
import { formatMediumDate, formatNumber } from '../lib/format'
import { notaAmount, notaRows } from '../lib/nota'
import { NotaShop } from './NotaShop'

// Surat jalan untuk kurir dan penerima, di kertas yang sama dengan nota pre-order: continuous form
// 9½ × 5½ inci untuk printer Epson LQ-310 (ukuran halamannya dipasang lewat @page di area cetak).
// Lengkap dengan harga dan rincian seperti nota PO, supaya penerima bisa mencocokkan tagihannya. Bayar ke Kurir =
// sisa yang ditagih kurir (0 dicetak LUNAS).
export function DeliverySlip({ delivery: d }: { delivery: Delivery }) {
  const items = d.items ?? []
  const rows = notaRows(items.length, 6)

  return (
    <div className="nota">
      <div className="nota-head">
        <NotaShop />
        <div className="nota-doc">
          <p className="nota-doc-title">SURAT JALAN</p>
          <dl className="nota-fields nota-fields-2">
            <dt>No. Surat Jalan</dt>
            <dd>{d.delivery_no}</dd>
            <dt>{d.source_type === 'preorder' ? 'No. PO' : 'No. Struk'}</dt>
            <dd>{d.source_no}</dd>
            <dt>Tgl. Kirim</dt>
            <dd>{formatMediumDate(d.scheduled_date)}</dd>
            <dt>Jam Kirim</dt>
            <dd>{d.scheduled_time ?? ''}</dd>
            <dt>Pemesan</dt>
            <dd>{d.customer_name}</dd>
            <dt>No. Telp</dt>
            <dd>{d.customer_phone ?? ''}</dd>
            <dt>Alamat</dt>
            <dd className="nota-wide">{d.address}</dd>
          </dl>
        </div>
      </div>

      <table className="nota-table" style={rows.style}>
        <colgroup>
          <col style={{ width: '28mm' }} />
          <col />
          <col style={{ width: '32mm' }} />
          <col style={{ width: '38mm' }} />
        </colgroup>
        <thead>
          <tr>
            <th>BANYAKNYA</th>
            <th>NAMA BARANG</th>
            <th>HARGA</th>
            <th>JUMLAH</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.product_name}>
              <td className="nota-qty">{formatNumber(item.qty)} pcs</td>
              <td>{item.product_name}</td>
              <td className="nota-num">{formatNumber(item.unit_price)}</td>
              <td className="nota-num">{formatNumber(item.line_total)}</td>
            </tr>
          ))}
          {Array.from({ length: rows.empty }, (_, i) => (
            <tr key={`kosong-${i}`} aria-hidden="true">
              <td />
              <td />
              <td />
              <td />
            </tr>
          ))}
        </tbody>
      </table>
      {d.notes && <p className="nota-note">Catatan: {d.notes}</p>}

      {/* Bawah tabel: tanda tangan di kiri (kolom Penerima ditandatangani orang yang menerima barangnya), rincian
          bayar di bawah HARGA dan JUMLAH (70 mm), seperti nota PO.
          Total = sub total + biaya lain + ongkir; Bayar ke Kurir = total - DP. */}
      <div className="nota-foot">
        <div className="nota-signs">
          <Sign label="Penerima" />
          <Sign label="Kurir" name={d.courier ?? ''} />
          <Sign label="Hormat Kami" name={d.created_by_name} />
        </div>
        <dl className="nota-totals">
          <div>
            <dt>Sub Total</dt>
            <dd>{notaAmount(d.subtotal)}</dd>
          </div>
          <div>
            <dt>Biaya Lain</dt>
            <dd>
              {d.other_fee > 0 && d.other_fee_note && <span className="nota-fee-note">{d.other_fee_note}</span>}
              {notaAmount(d.other_fee)}
            </dd>
          </div>
          <div>
            <dt>Ongkir</dt>
            <dd>{notaAmount(d.fee)}</dd>
          </div>
          <div>
            <dt>Total</dt>
            <dd>{notaAmount(d.total)}</dd>
          </div>
          <div>
            <dt>Sudah DP</dt>
            <dd>{notaAmount(d.down_payment)}</dd>
          </div>
          <div className="nota-final">
            <dt>Bayar ke Kurir</dt>
            <dd>{d.amount_due > 0 ? `Rp ${formatNumber(d.amount_due)}` : 'LUNAS'}</dd>
          </div>
        </dl>
      </div>
    </div>
  )
}

function Sign({ label, name = '' }: { label: string; name?: string }) {
  return (
    <div className="nota-sign">
      <p>{label}</p>
      <p className="nota-sign-name">{name}</p>
    </div>
  )
}
