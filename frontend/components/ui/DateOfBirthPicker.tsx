"use client"

function daysInMonth(month: number, year: number): number {
  if (!month) return 31
  return new Date(year || 2000, month, 0).getDate()
}

interface Labels {
  month?: string
  day?: string
  year?: string
}

interface Props {
  id?: string
  label?: string
  value: string
  onChange: (v: string) => void
  onBlur?: () => void
  error?: string
  labels?: Labels
}

export default function DateOfBirthPicker({ id, label, value, onChange, onBlur, error, labels }: Props) {
  const parts  = value.split("-")
  const year   = parts[0] ? parseInt(parts[0], 10) : 0
  const month  = parts[1] ? parseInt(parts[1], 10) : 0
  const day    = parts[2] ? parseInt(parts[2], 10) : 0

  const now       = new Date()
  const maxYear   = now.getFullYear() - 18
  const minYear   = now.getFullYear() - 100
  const totalDays = daysInMonth(month, year)

  function emit(y: number, m: number, d: number) {
    if (!y || !m || !d) { onChange(""); return }
    const maxD    = daysInMonth(m, y)
    const clamped = Math.min(d, maxD)
    onChange(
      `${String(y).padStart(4, "0")}-${String(m).padStart(2, "0")}-${String(clamped).padStart(2, "0")}`
    )
  }

  const baseSelect =
    "min-w-0 flex-1 rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white shadow-sm " +
    "focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"

  const dayLabel   = labels?.day   ?? "DD"
  const monthLabel = labels?.month ?? "MM"
  const yearLabel  = labels?.year  ?? "AAAA"

  return (
    <div>
      {label && (
        <label className="block text-sm font-medium text-gray-300">{label}</label>
      )}
      <div className={`flex gap-2${label ? " mt-1" : ""}`}>
        <select
          aria-label={dayLabel}
          value={day || ""}
          onChange={(e) => emit(year, month, parseInt(e.target.value, 10) || 0)}
          onBlur={onBlur}
          className={baseSelect}
        >
          <option value="">{dayLabel}</option>
          {Array.from({ length: totalDays }, (_, i) => i + 1).map((d) => (
            <option key={d} value={d}>{String(d).padStart(2, "0")}</option>
          ))}
        </select>

        <select
          aria-label={monthLabel}
          value={month || ""}
          onChange={(e) => emit(year, parseInt(e.target.value, 10) || 0, day)}
          onBlur={onBlur}
          className={baseSelect}
        >
          <option value="">{monthLabel}</option>
          {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
            <option key={m} value={m}>{String(m).padStart(2, "0")}</option>
          ))}
        </select>

        <select
          id={id}
          aria-label={yearLabel}
          value={year || ""}
          onChange={(e) => emit(parseInt(e.target.value, 10) || 0, month, day)}
          onBlur={onBlur}
          className={baseSelect}
        >
          <option value="">{yearLabel}</option>
          {Array.from({ length: maxYear - minYear + 1 }, (_, i) => maxYear - i).map((y) => (
            <option key={y} value={y}>{y}</option>
          ))}
        </select>
      </div>
      {error && (
        <p className="mt-1 text-xs text-red-400" role="alert">{error}</p>
      )}
    </div>
  )
}
