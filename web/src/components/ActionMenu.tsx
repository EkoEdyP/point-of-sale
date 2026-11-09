import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'

export interface MenuAction {
  label: string
  icon?: ReactNode
  danger?: boolean
  onSelect: () => void
}

// 'separator' = garis pemisah, misalnya sebelum aksi berbahaya seperti Batalkan.
export type MenuEntry = MenuAction | 'separator'

const EDGE = 8 // jarak minimal menu dari tepi layar (px)
const GAP = 6 // jarak menu dari tombolnya (px)

// Tombol "Lainnya" yang membuka daftar aksi tambahan.
// Memakai Popover API bawaan browser: menu tampil di lapisan paling atas sehingga tidak terpotong scroll dialog,
// klik di luar menutup menu, dan Escape hanya menutup menu (dialog di belakangnya tetap terbuka).
export function ActionMenu({
  label = 'Lainnya',
  entries,
  className = '',
}: {
  label?: string
  entries: MenuEntry[]
  className?: string
}) {
  const triggerId = useId()
  const menuId = useId()
  const triggerRef = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    const menu = menuRef.current
    const trigger = triggerRef.current
    if (!menu || !trigger) return

    // Sembunyikan dulu sampai posisinya dihitung, supaya menu tidak sempat berkedip di pojok layar.
    function beforeToggle(e: Event) {
      if (!menu || (e as ToggleEvent).newState !== 'open') return
      menu.style.visibility = 'hidden'
      menu.style.maxHeight = ''
    }

    function toggle(e: Event) {
      if (!menu || !trigger) return
      const isOpen = (e as ToggleEvent).newState === 'open'
      setOpen(isOpen)
      if (!isOpen) return
      const t = trigger.getBoundingClientRect()
      const m = menu.getBoundingClientRect()
      const vw = document.documentElement.clientWidth
      const vh = document.documentElement.clientHeight
      const above = t.top - GAP - EDGE
      const below = vh - t.bottom - GAP - EDGE
      // Utamakan membuka ke atas: tombolnya ada di baris aksi paling bawah dialog.
      const up = m.height <= above || above > below
      const maxHeight = Math.max(up ? above : below, 120)
      const height = Math.min(m.height, maxHeight)
      menu.style.maxHeight = `${maxHeight}px`
      menu.style.top = `${up ? t.top - GAP - height : t.bottom + GAP}px`
      menu.style.left = `${Math.max(EDGE, Math.min(t.left, vw - EDGE - m.width))}px`
      menu.style.visibility = ''
      menu.querySelector<HTMLElement>('[role="menuitem"]')?.focus({ preventScroll: true })
    }

    menu.addEventListener('beforetoggle', beforeToggle)
    menu.addEventListener('toggle', toggle)
    return () => {
      menu.removeEventListener('beforetoggle', beforeToggle)
      menu.removeEventListener('toggle', toggle)
    }
  }, [])

  // Posisi menu tetap di layar: kalau dialog di-scroll atau ukuran layar berubah, menu ditutup saja
  // daripada melayang terpisah dari tombolnya.
  useEffect(() => {
    if (!open) return
    function close(e: Event) {
      const menu = menuRef.current
      if (!menu || (e.target instanceof Node && menu.contains(e.target))) return
      hide()
    }
    window.addEventListener('scroll', close, true)
    window.addEventListener('resize', close)
    return () => {
      window.removeEventListener('scroll', close, true)
      window.removeEventListener('resize', close)
    }
  }, [open])

  function hide() {
    const menu = menuRef.current
    if (menu?.matches(':popover-open')) menu.hidePopover()
  }

  // Fokus dikembalikan ke tombol "Lainnya" sebelum menu ditutup, jadi dialog yang dibuka aksi berikutnya
  // mengembalikan fokus ke tombol itu saat ditutup.
  function focusTrigger() {
    triggerRef.current?.focus({ preventScroll: true })
  }

  function select(action: MenuAction) {
    focusTrigger()
    hide()
    action.onSelect()
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const items = Array.from(menuRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])
    if (items.length === 0) return
    const current = items.indexOf(document.activeElement as HTMLElement)
    let next: number
    switch (e.key) {
      case 'ArrowDown':
        next = current < 0 ? 0 : (current + 1) % items.length
        break
      case 'ArrowUp':
        next = current < 0 ? items.length - 1 : (current - 1 + items.length) % items.length
        break
      case 'Home':
        next = 0
        break
      case 'End':
        next = items.length - 1
        break
      case 'Escape':
        // Ditangani di sini supaya Escape tidak ikut menutup dialog di belakang menu.
        e.preventDefault()
        e.stopPropagation()
        focusTrigger()
        hide()
        return
      case 'Tab':
        // Tab keluar dari menu: menu ditutup, fokus lanjut dari tombol "Lainnya" ke tombol berikutnya.
        focusTrigger()
        hide()
        return
      default:
        return
    }
    e.preventDefault()
    items[next].focus()
  }

  return (
    <>
      <button
        ref={triggerRef}
        id={triggerId}
        type="button"
        popoverTarget={menuId}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        className={`inline-flex items-center justify-center gap-1.5 aria-expanded:bg-ink aria-expanded:text-white ${className}`}
      >
        {label}
        <svg
          aria-hidden="true"
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2.25"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={`shrink-0 transition-transform ${open ? 'rotate-180' : ''}`}
        >
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>
      <div
        ref={menuRef}
        id={menuId}
        popover="auto"
        role="menu"
        aria-labelledby={triggerId}
        onKeyDown={onKeyDown}
        className="inset-auto m-0 min-w-60 max-w-[calc(100vw-1rem)] overflow-y-auto p-1.5 bg-white text-ink border-2 border-ink rounded-sm shadow-[3px_3px_0_var(--color-ink)]"
      >
        {entries.map((entry, i) =>
          entry === 'separator' ? (
            <div key={`sep-${i}`} role="separator" className="my-1.5 border-t border-line" />
          ) : (
            <button
              key={entry.label}
              type="button"
              role="menuitem"
              tabIndex={-1}
              onClick={() => select(entry)}
              className={`flex w-full h-11 pointer-fine:h-10 items-center gap-2.5 px-3 text-left font-semibold whitespace-nowrap rounded-sm cursor-pointer hover:bg-wall focus-visible:bg-wall focus-visible:outline-offset-[-3px] ${
                entry.danger ? 'text-merah' : ''
              }`}
            >
              {entry.icon}
              {entry.label}
            </button>
          ),
        )}
      </div>
    </>
  )
}
