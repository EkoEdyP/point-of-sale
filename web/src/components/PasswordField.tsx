import { useState } from 'react'

// Tanpa huruf yang mudah tertukar (0/O, 1/l/I) supaya password acak mudah dibacakan.
const ALPHABET = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789'

export function randomPassword(length = 10): string {
  const limit = 256 - (256 % ALPHABET.length) // buang nilai di atas batas supaya sebaran huruf merata
  const result: string[] = []
  while (result.length < length) {
    const bytes = new Uint8Array(length)
    crypto.getRandomValues(bytes)
    for (const b of bytes) {
      if (b < limit && result.length < length) result.push(ALPHABET[b % ALPHABET.length])
    }
  }
  return result.join('')
}

export function PasswordField({
  id,
  label,
  value,
  onChange,
}: {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
}) {
  const [visible, setVisible] = useState(false)

  return (
    <div>
      <div className="flex items-baseline justify-between gap-3">
        <label htmlFor={id} className="block mb-1 text-sm font-semibold">
          {label}
        </label>
        <span className="flex gap-3">
          <button
            type="button"
            onClick={() => {
              onChange(randomPassword())
              setVisible(true)
            }}
            className="py-2 -my-2 text-sm font-semibold underline underline-offset-2 cursor-pointer"
          >
            Buat acak
          </button>
          <button
            type="button"
            onClick={() => setVisible((v) => !v)}
            className="py-2 -my-2 text-sm font-semibold underline underline-offset-2 cursor-pointer"
          >
            {visible ? 'Sembunyikan' : 'Tampilkan'}
          </button>
        </span>
      </div>
      <input
        id={id}
        type={visible ? 'text' : 'password'}
        autoComplete="new-password"
        autoCapitalize="none"
        spellCheck={false}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        maxLength={72}
        className="w-full h-11 px-3 bg-white border-2 border-ink rounded-sm font-mono"
      />
      <p className="mt-1 text-sm text-ink-soft">Minimal 8 karakter.</p>
    </div>
  )
}
