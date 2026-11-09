import type { ReactNode } from 'react'

// Ikon kecil untuk item menu "Lainnya" (ActionMenu) di dialog detail pre-order dan pengiriman.
function ActionIcon({ children }: { children: ReactNode }) {
  return (
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
      className="shrink-0"
    >
      {children}
    </svg>
  )
}

export const PrintIcon = () => (
  <ActionIcon>
    <path d="M6 9V3h12v6" />
    <rect x="3" y="9" width="18" height="8" rx="1" />
    <path d="M6 14h12v7H6z" />
  </ActionIcon>
)

export const PlusIcon = () => (
  <ActionIcon>
    <path d="M12 5v14M5 12h14" />
  </ActionIcon>
)

export const CalendarIcon = () => (
  <ActionIcon>
    <rect x="3" y="5" width="18" height="16" rx="1" />
    <path d="M3 10h18M8 3v4M16 3v4" />
  </ActionIcon>
)

export const UserIcon = () => (
  <ActionIcon>
    <circle cx="12" cy="8" r="4" />
    <path d="M4 21c0-4 3.6-7 8-7s8 3 8 7" />
  </ActionIcon>
)

export const EditIcon = () => (
  <ActionIcon>
    <path d="M4 20h4L19 9l-4-4L4 16v4z" />
    <path d="m14 6 4 4" />
  </ActionIcon>
)

export const AlertIcon = () => (
  <ActionIcon>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 7.5V13M12 16.5v.01" />
  </ActionIcon>
)

export const HistoryIcon = () => (
  <ActionIcon>
    <path d="M3.5 12a8.5 8.5 0 1 0 2.6-6.1" />
    <path d="M3.5 4.5v4.5H8" />
    <path d="M12 7.5V12l3 2" />
  </ActionIcon>
)

export const CancelIcon = () => (
  <ActionIcon>
    <path d="M6 6l12 12M18 6 6 18" />
  </ActionIcon>
)
