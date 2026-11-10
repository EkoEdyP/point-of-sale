import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Preorder } from '../api/types'
import { formatFullDate, formatNumber } from '../lib/format'
import { NOTA_PAGE_CSS, NOTA_PAGE_W, notaAmount as amount, notaRows } from '../lib/nota'
import { Modal } from './Modal'
import { NotaShop } from './NotaShop'

// Nota pre-order untuk pelanggan: bukti pesanan dan DP, mengikuti nota cetakan toko.
// Tanggal dan jam adalah jadwal ambil atau antar (hari H).
export function PreorderNota({ preorder: p }: { preorder: Preorder }) {
  const items = p.items ?? []
  // Catatan (paling banyak 2 baris) memakai tempat dua baris tabel, supaya nota tetap satu lembar.
  const { empty: emptyRows, style: rowStyle } = notaRows(items.length, p.notes ? 5 : 7)
  // PO yang sudah diserahkan sudah dilunasi di struk penyerahannya.
  const unpaid = p.status === 'completed' ? 0 : p.remaining

  return (
    <div className="nota">
      <div className="nota-head">
        <NotaShop />
        <dl className="nota-fields">
          <dt>No. PO</dt>
          <dd>{p.order_no}</dd>
          <dt>Tanggal</dt>
          <dd>{formatFullDate(p.due_date)}</dd>
          <dt>Jam</dt>
          <dd>{p.due_time ?? ''}</dd>
          <dt>Nama</dt>
          <dd>{p.customer_name}</dd>
          <dt>No Telp</dt>
          <dd>{p.customer_phone ?? ''}</dd>
          <dt>Alamat</dt>
          <dd>{p.fulfillment === 'delivery' ? (p.address ?? '') : 'Ambil di toko'}</dd>
        </dl>
      </div>

      <table className="nota-table" style={rowStyle}>
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
            <tr key={item.id}>
              <td className="nota-num nota-qty">{formatNumber(item.qty)}</td>
              <td>{item.product_name}</td>
              <td className="nota-num">{formatNumber(item.unit_price)}</td>
              <td className="nota-num">{formatNumber(item.line_total)}</td>
            </tr>
          ))}
          {Array.from({ length: emptyRows }, (_, i) => (
            <tr key={`kosong-${i}`} aria-hidden="true">
              <td />
              <td />
              <td />
              <td />
            </tr>
          ))}
        </tbody>
      </table>
      {p.notes && <p className="nota-note">Catatan: {p.notes}</p>}

      {/* Bawah tabel, seperti nota toko: rincian bayar di bawah HARGA dan JUMLAH. Sisa Bayar = harga barang - DP;
          Total Akhir = yang masih dibayar pelanggan (sudah termasuk biaya lain dan ongkir). */}
      <div className="nota-foot">
        <div className="nota-signs">
          <div className="nota-sign">
            <p>Tanda Terima</p>
            <p className="nota-sign-name" />
          </div>
          <div className="nota-sign">
            <p>Hormat Kami</p>
            <p className="nota-sign-name">{p.created_by_name}</p>
          </div>
        </div>
        <dl className="nota-totals">
          <div>
            <dt>Sub Total</dt>
            <dd>{amount(p.subtotal)}</dd>
          </div>
          <div>
            <dt>Dp</dt>
            <dd>{amount(p.down_payment)}</dd>
          </div>
          <div>
            <dt>Sisa Bayar</dt>
            <dd>{amount(Math.max(p.subtotal - p.down_payment, 0))}</dd>
          </div>
          <div>
            <dt>Biaya Lain</dt>
            <dd>
              {p.other_fee > 0 && p.other_fee_note && <span className="nota-fee-note">{p.other_fee_note}</span>}
              {amount(p.other_fee)}
            </dd>
          </div>
          <div>
            <dt>Ongkir</dt>
            <dd>{amount(p.shipping_fee)}</dd>
          </div>
          <div className="nota-final">
            <dt>Total Akhir</dt>
            <dd>{unpaid > 0 ? formatNumber(unpaid) : 'LUNAS'}</dd>
          </div>
        </dl>
      </div>
    </div>
  )
}

// Pratinjau nota (diperkecil mengikuti lebar dialog) dan tombol cetak.
export function PreorderNotaDialog({ preorder, onClose }: { preorder: Preorder; onClose: () => void }) {
  return (
    <>
      <Modal title={`Nota ${preorder.order_no}`} onClose={onClose} size="wide">
        <ScaledNota preorder={preorder} />
        <div className="mt-4 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            className="h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer"
          >
            Tutup
          </button>
          <button
            type="button"
            onClick={() => window.print()}
            className="h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer"
          >
            Cetak nota
          </button>
        </div>
      </Modal>

      {/* Satu-satunya yang tampil saat mencetak (lihat index.css), dengan ukuran halaman nota. */}
      {createPortal(
        <div id="print-area">
          <style>{NOTA_PAGE_CSS}</style>
          <PreorderNota preorder={preorder} />
        </div>,
        document.body,
      )}
    </>
  )
}

function ScaledNota({ preorder }: { preorder: Preorder }) {
  const frame = useRef<HTMLDivElement>(null)
  const page = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState({ scale: 0, height: 0 })

  // Lebar dialog dan tinggi nota (bisa lebih dari satu lembar) diukur ulang setiap kali berubah.
  useLayoutEffect(() => {
    const el = frame.current
    const inner = page.current
    if (!el || !inner) return
    const update = () => {
      const scale = Math.min(1, el.clientWidth / NOTA_PAGE_W)
      setSize({ scale, height: inner.offsetHeight * scale })
    }
    update()
    const observer = new ResizeObserver(update)
    observer.observe(el)
    observer.observe(inner)
    return () => observer.disconnect()
  }, [])

  return (
    <div ref={frame} className="mt-4 overflow-hidden border border-line bg-white" style={{ height: size.height }}>
      <div ref={page} style={{ width: NOTA_PAGE_W, transform: `scale(${size.scale})`, transformOrigin: 'top left' }}>
        <PreorderNota preorder={preorder} />
      </div>
    </div>
  )
}
