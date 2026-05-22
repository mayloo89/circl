"use client"

interface RangeSliderProps {
  min: number
  max: number
  value: number
  onChange: (value: number) => void
  formatValue?: (value: number) => string
  disabled?: boolean
  className?: string
  /** Accessible label describing what the slider controls (e.g. "Minimum age"). Required for screen readers. */
  ariaLabel?: string
}

export default function RangeSlider({
  min,
  max,
  value,
  onChange,
  formatValue,
  disabled,
  className = "",
  ariaLabel,
}: RangeSliderProps) {
  const pct = max > min ? ((value - min) / (max - min)) * 100 : 0
  const display = formatValue ? formatValue(value) : String(value)

  return (
    <div className={`flex items-center gap-3 ${className}`}>
      <div className="relative flex-1 flex h-5 items-center">
        <div className="pointer-events-none absolute inset-x-0 h-1.5 rounded-full bg-gray-700">
          <div
            className="h-full rounded-full bg-brand-primary transition-[width] duration-75"
            style={{ width: `${pct}%` }}
          />
        </div>
        <input
          type="range"
          min={min}
          max={max}
          value={value}
          disabled={disabled}
          aria-label={ariaLabel}
          aria-valuetext={display}
          onChange={(e) => onChange(Number(e.target.value))}
          className="relative w-full cursor-pointer appearance-none bg-transparent
            [&::-webkit-slider-thumb]:appearance-none
            [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4
            [&::-webkit-slider-thumb]:rounded-full
            [&::-webkit-slider-thumb]:bg-brand-primary
            [&::-webkit-slider-thumb]:ring-2 [&::-webkit-slider-thumb]:ring-gray-900
            [&::-webkit-slider-thumb]:transition-transform
            [&::-webkit-slider-thumb:active]:scale-125
            [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4
            [&::-moz-range-thumb]:rounded-full
            [&::-moz-range-thumb]:bg-brand-primary
            [&::-moz-range-thumb]:border-[3px] [&::-moz-range-thumb]:border-gray-900
            [&::-moz-range-thumb]:cursor-pointer
            disabled:opacity-40 disabled:cursor-not-allowed"
        />
      </div>
      <span className="flex-none w-14 text-right text-sm font-medium text-foreground tabular-nums whitespace-nowrap">
        {display}
      </span>
    </div>
  )
}
