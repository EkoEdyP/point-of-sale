import notaLogo from '../assets/logo-fn-boss-nota.png'
import { STORE } from '../config'

// Blok toko di kiri atas nota pre-order dan surat jalan, meniru nota cetakan toko.
export function NotaShop() {
  return (
    <div className="nota-shop">
      <p className="nota-title">{STORE.notaTitle}</p>
      <img className="nota-logo" src={notaLogo} width={279} height={269} alt={STORE.name} />
      <p className="nota-phone">{STORE.phone}</p>
      <p>{STORE.address}</p>
      <p>{STORE.bankAccount}</p>
    </div>
  )
}
