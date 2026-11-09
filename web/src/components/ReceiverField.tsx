// Nama orang yang menerima barang pre-order diantar, wajib diisi saat menandai terkirim. Sering pemesannya sendiri,
// jadi namanya bisa diisi dengan satu ketukan.
export function ReceiverField({
  value,
  onChange,
  customerName,
  autoFocus = false,
}: {
  value: string
  onChange: (value: string) => void
  customerName: string
  autoFocus?: boolean
}) {
  return (
    <div>
      <label htmlFor="received-by" className="block mb-1 text-sm font-semibold">
        Nama penerima
      </label>
      <input
        id="received-by"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        maxLength={100}
        autoComplete="off"
        autoFocus={autoFocus}
        aria-describedby="received-by-note"
        className="w-full h-11 px-3 bg-white border-2 border-ink rounded-sm"
      />
      <p id="received-by-note" className="mt-1 text-sm text-ink-soft">
        Orang yang menerima barangnya, mis. pemesan sendiri, keluarga, atau satpam.
      </p>
      <button
        type="button"
        onClick={() => onChange(customerName)}
        className="mt-2 min-h-10 max-w-full px-3 py-1 border-2 border-ink rounded-sm text-sm font-semibold text-left break-words cursor-pointer"
      >
        Pemesan sendiri: {customerName}
      </button>
    </div>
  )
}
