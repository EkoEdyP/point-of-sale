import { formatNumber } from '../lib/format'

const buttonClass =
  'flex-1 sm:flex-none h-11 sm:h-10 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed'

// Navigasi halaman di bawah daftar. Di layar sempit tombolnya dibuat selebar layar supaya mudah diketuk.
export function Pagination({
  page,
  pages,
  total,
  noun,
  onPage,
}: {
  page: number
  pages: number
  total: number
  noun: string
  onPage: (page: number) => void
}) {
  if (total <= 0) return null
  return (
    <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
      <p className="text-sm text-ink-soft">
        Halaman {page} dari {pages}, {formatNumber(total)} {noun}
      </p>
      <div className="flex w-full sm:w-auto gap-2">
        <button type="button" disabled={page <= 1} onClick={() => onPage(page - 1)} className={buttonClass}>
          Sebelumnya
        </button>
        <button type="button" disabled={page >= pages} onClick={() => onPage(page + 1)} className={buttonClass}>
          Berikutnya
        </button>
      </div>
    </div>
  )
}
