import { useEffect, useId, useRef, type MouseEvent, type ReactNode } from 'react'

// Dialog bawaan browser: fokus terkunci di dalam dialog dan Escape menutupnya.
// Komponen ini dipasang saat dibutuhkan dan dilepas saat ditutup. Selama busy (sedang menyimpan) dialog tidak bisa
// ditutup dan semua isian serta tombolnya nonaktif, supaya data yang sama tidak terkirim dua kali.
export function Modal({
  title,
  onClose,
  children,
  size = 'normal',
  busy = false,
}: {
  title: string
  onClose: () => void
  children: ReactNode
  size?: 'normal' | 'wide'
  busy?: boolean
}) {
  const ref = useRef<HTMLDialogElement>(null)
  const titleId = useId()
  // Klik di area gelap menutup dialog hanya kalau tekanannya juga dimulai di sana. Seleksi teks yang dilepas di luar
  // kotak, atau klik di scrollbar dialog, tidak menutupnya.
  const pressedOutside = useRef(false)

  useEffect(() => {
    const dialog = ref.current
    if (dialog && !dialog.open) dialog.showModal()
  }, [])

  function outsideBox(e: MouseEvent) {
    const box = ref.current?.getBoundingClientRect()
    if (!box) return false
    return e.clientX < box.left || e.clientX > box.right || e.clientY < box.top || e.clientY > box.bottom
  }

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      aria-busy={busy}
      onCancel={(e) => {
        if (busy) e.preventDefault()
      }}
      onClose={() => {
        // Browser tetap menutup dialog kalau Escape ditekan berulang walau cancel dicegah; selama busy, buka lagi.
        if (busy) ref.current?.showModal()
        else onClose()
      }}
      onPointerDown={(e) => {
        pressedOutside.current = e.target === ref.current && outsideBox(e)
      }}
      onClick={(e) => {
        const close = pressedOutside.current && e.target === ref.current && outsideBox(e) && !busy
        pressedOutside.current = false
        if (close) onClose()
      }}
      className={`m-auto ${
        size === 'wide' ? 'w-[min(44rem,calc(100%-2rem))]' : 'w-[min(30rem,calc(100%-2rem))]'
      } max-h-[calc(100dvh-2rem)] overflow-y-auto bg-paper text-ink border-2 border-ink rounded-sm p-0 backdrop:bg-ink/50`}
    >
      <div className="p-4 sm:p-6">
        <h2 id={titleId} className="text-xl font-bold">
          {title}
        </h2>
        <fieldset disabled={busy} className="min-w-0">
          {children}
        </fieldset>
      </div>
    </dialog>
  )
}
