import type { AuditEntry } from '../api/types'
import { describeAudit } from '../lib/audit'

// Isi satu perubahan: judul aksi (misalnya Dibatalkan), kolom yang berubah dengan nilai lama dicoret lalu nilai
// barunya, dan catatan atau alasannya.
export function AuditEntryView({ entry }: { entry: AuditEntry }) {
  const d = describeAudit(entry)
  return (
    <div className="min-w-0 break-words">
      {d.title && <p className={`font-semibold ${d.danger ? 'text-merah' : ''}`}>{d.title}</p>}
      {d.lines.length > 0 && (
        <ul className="space-y-0.5">
          {d.lines.map((line, i) => (
            <li key={i}>
              {line.kind === 'text' ? (
                line.text
              ) : (
                <>
                  <span className="font-semibold">{line.label}</span>{' '}
                  {line.kind === 'value' ? (
                    line.value
                  ) : (
                    <>
                      <span className="text-ink-soft line-through">{line.before}</span>
                      <span aria-hidden="true"> → </span>
                      <span className="sr-only"> menjadi </span>
                      {line.after}
                    </>
                  )}
                </>
              )}
            </li>
          ))}
        </ul>
      )}
      {d.note && <p className="text-sm text-ink-soft">{d.note}</p>}
    </div>
  )
}
