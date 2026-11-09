const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

const number = new Intl.NumberFormat('id-ID')

// Pesan dari server diawali huruf kecil; untuk kalimat yang disusun di layar huruf pertamanya dibesarkan.
export function sentence(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1)
}

export function formatRupiah(value: number): string {
  return rupiah.format(value)
}

export function formatNumber(value: number): string {
  return number.format(value)
}

const dateTime = new Intl.DateTimeFormat('id-ID', { dateStyle: 'short', timeStyle: 'short' })

export function formatDateTime(iso: string): string {
  return dateTime.format(new Date(iso))
}

const time = new Intl.DateTimeFormat('id-ID', { hour: '2-digit', minute: '2-digit' })

const pad = (n: number) => String(n).padStart(2, '0')

// Format YYYY-MM-DD (zona waktu perangkat) untuk <input type="date"> dan parameter API.
export function toDateInput(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

export function addDays(date: Date, days: number): Date {
  const copy = new Date(date)
  copy.setDate(copy.getDate() + days)
  return copy
}

// "2026-10-22" menjadi tanggal lokal (bukan UTC), supaya hari tidak bergeser.
export function parseDateInput(value: string): Date {
  const [y, m, d] = value.split('-').map(Number)
  return new Date(y, m - 1, d)
}

const longDate = new Intl.DateTimeFormat('id-ID', { dateStyle: 'long' })
const shortDay = new Intl.DateTimeFormat('id-ID', { weekday: 'short', day: 'numeric', month: 'short' })
const weekdayDate = new Intl.DateTimeFormat('id-ID', { weekday: 'long', day: 'numeric', month: 'long' })

export function formatLongDate(value: string): string {
  return longDate.format(parseDateInput(value))
}

export function formatShortDay(value: string): string {
  return shortDay.format(parseDateInput(value))
}

const shortDayYear = new Intl.DateTimeFormat('id-ID', {
  weekday: 'short',
  day: 'numeric',
  month: 'short',
  year: 'numeric',
})

// Waktu sebuah kejadian, misalnya "Sel, 3 Nov · 14.05". Tahun ditulis kalau bukan tahun ini.
export function formatDayTime(iso: string): string {
  const d = new Date(iso)
  const day = d.getFullYear() === new Date().getFullYear() ? shortDay.format(d) : shortDayYear.format(d)
  return `${day} · ${time.format(d)}`
}

// "2026-10-30" menjadi "Kamis, 30 Oktober".
export function formatWeekdayDate(value: string): string {
  return weekdayDate.format(parseDateInput(value))
}

const fullDate = new Intl.DateTimeFormat('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })

// "2026-11-02" menjadi "Senin, 2 November 2026".
export function formatFullDate(value: string): string {
  return fullDate.format(parseDateInput(value))
}

const mediumDate = new Intl.DateTimeFormat('id-ID', {
  weekday: 'long',
  day: 'numeric',
  month: 'short',
  year: 'numeric',
})

// "2026-11-03" menjadi "Selasa, 3 Nov 2026" (surat jalan: kolomnya lebih sempit dari nota PO).
export function formatMediumDate(value: string): string {
  return mediumDate.format(parseDateInput(value))
}
