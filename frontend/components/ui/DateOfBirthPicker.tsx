"use client"

import { useState } from "react"

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

function parseParts(value: string): [number, number, number] {
  const parts = value.split("-")
  return [
    parts[0] ? parseInt(parts[0], 10) : 0,
    parts[1] ? parseInt(parts[1], 10) : 0,
    parts[2] ? parseInt(parts[2], 10) : 0,
  ]
}

export default function DateOfBirthPicker({ id, label, value, onChange, onBlur, error, labels }: Props) {
  const [y0, m0, d0] = parseParts(value)
  const [localYear,  setLocalYear]  = useState(y0)
  const [localMonth, setLocalMonth] = useState(m0)
  const [localDay,   setLocalDay]   = useState(d0)

  function emit(y: number, m: number, d: number) {
    if (!y || !m || !d) { onChange(""); return }
    const maxD    = daysInMonth(m, y)
    const clamped = Math.min(d, maxD)
    onChange(
      `${String(y).padStart(4, "0")}-${String(m).padStart(2, "0")}-${String(clamped).padStart(2, "0")}`
    )
  }

  function handleDay(d: number) {
    setLocalDay(d)
    emit(localYear, localMonth, d)
  }

  function handleMonth(m: number) {
    const maxD    = daysInMonth(m, localYear)
    const clamped = localDay ? Math.min(localDay, maxD) : localDay
    setLocalMonth(m)
    if (clamped !== localDay) setLocalDay(clamped)
    emit(localYear, m, clamped)
  }

  function handleYear(y: number) {
    setLocalYear(y)
    emit(y, localMonth, localDay)
  }

  const now      = new Date()
  const maxYear  = now.getFullYear() - 18
  const minYear  = now.getFullYear() - 100
  const totalDays = daysInMonth(localMonth, localYear)

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
          value={localDay || ""}
          onChange={(e) => handleDay(parseInt(e.target.value, 10) || 0)}
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
          value={localMonth || ""}
          onChange={(e) => handleMonth(parseInt(e.target.value, 10) || 0)}
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
          value={localYear || ""}
          onChange={(e) => handleYear(parseInt(e.target.value, 10) || 0)}
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
