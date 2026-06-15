type RingProps = {
  className: string
  duration: number
  reverse?: boolean
  children: React.ReactNode
}

// A rotating ring layer. Rotation runs on the wrapping div (centred origin is
// trivial) so the ring stroke and the presence dots riding on it turn together.
function Ring({ className, duration, reverse, children }: RingProps) {
  return (
    <div
      className={`absolute ${className}`}
      style={{
        animation: `${reverse ? "landing-orbit-reverse" : "landing-orbit"} ${duration}s linear infinite`,
      }}
    >
      <svg viewBox="0 0 400 400" className="h-full w-full" aria-hidden="true">
        {children}
      </svg>
    </div>
  )
}

function Dot({ cx, cy, r, fill }: { cx: number; cy: number; r: number; fill: string }) {
  return (
    <g>
      <circle cx={cx} cy={cy} r={r * 2.6} fill={fill} opacity={0.18} />
      <circle cx={cx} cy={cy} r={r} fill={fill} />
    </g>
  )
}

// Decorative identity motif: concentric orbits of presence around a central
// node, with two drifting message bubbles. Pure presentation, no semantics.
export default function ConnectionMotif() {
  return (
    <div
      aria-hidden="true"
      className="relative mx-auto aspect-square w-full max-w-[34rem] select-none"
    >
      {/* Aurora wash — brand blue + rose, soft and drifting */}
      <div
        className="absolute left-[8%] top-[6%] h-[60%] w-[60%] rounded-full blur-3xl"
        style={{
          background: "radial-gradient(circle, var(--brand-primary), transparent 70%)",
          opacity: 0.45,
          animation: "landing-aurora 14s ease-in-out infinite",
        }}
      />
      <div
        className="absolute bottom-[6%] right-[8%] h-[58%] w-[58%] rounded-full blur-3xl"
        style={{
          background: "radial-gradient(circle, var(--brand-accent), transparent 70%)",
          opacity: 0.4,
          animation: "landing-aurora 18s ease-in-out infinite 2s",
        }}
      />

      {/* Outer orbit */}
      <Ring className="inset-0" duration={52}>
        <circle cx="200" cy="200" r="180" fill="none" stroke="var(--brand-muted)" strokeOpacity="0.35" strokeWidth="1.5" />
        <Dot cx={200} cy={20} r={6} fill="var(--brand-accent)" />
        <Dot cx={356} cy={290} r={4} fill="var(--brand-subtle)" />
        <Dot cx={44} cy={290} r={5} fill="var(--brand-strong)" />
      </Ring>

      {/* Mid orbit */}
      <Ring className="inset-[15%]" duration={38} reverse>
        <circle cx="200" cy="200" r="180" fill="none" stroke="var(--brand-subtle)" strokeOpacity="0.4" strokeWidth="1.5" />
        <Dot cx={200} cy={20} r={5} fill="var(--brand-subtle)" />
        <Dot cx={20} cy={200} r={6} fill="var(--brand-accent)" />
      </Ring>

      {/* Inner orbit */}
      <Ring className="inset-[30%]" duration={28}>
        <circle cx="200" cy="200" r="180" fill="none" stroke="var(--brand-accent)" strokeOpacity="0.5" strokeWidth="2" strokeDasharray="3 8" />
        <Dot cx={380} cy={200} r={5} fill="var(--brand-accent)" />
      </Ring>

      {/* Central node */}
      <div className="absolute left-1/2 top-1/2 flex h-[22%] w-[22%] -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full bg-gradient-to-br from-brand-primary to-brand-accent shadow-[0_8px_40px_-8px_var(--brand-accent)]">
        <span className="h-3 w-3 rounded-full bg-white/90 shadow-[0_0_12px_2px_rgba(255,255,255,0.7)]" />
      </div>

      {/* Drifting message bubbles */}
      <div
        className="absolute left-[2%] top-[34%] rounded-2xl rounded-bl-sm bg-brand-surface-elevated px-3 py-2.5 shadow-card ring-1 ring-brand-light/60"
        style={{ animation: "landing-float 6s ease-in-out infinite" }}
      >
        <span className="block h-1.5 w-12 rounded-full bg-brand-muted/60" />
        <span className="mt-1.5 block h-1.5 w-8 rounded-full bg-brand-muted/40" />
      </div>
      <div
        className="absolute bottom-[20%] right-[1%] rounded-2xl rounded-br-sm bg-brand-accent px-3 py-2.5 shadow-card"
        style={{ animation: "landing-float 7s ease-in-out infinite 1.5s" }}
      >
        <span className="block h-1.5 w-10 rounded-full bg-white/80" />
        <span className="mt-1.5 block h-1.5 w-14 rounded-full bg-white/55" />
      </div>
    </div>
  )
}
