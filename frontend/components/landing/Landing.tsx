"use client"

import { useTranslations } from "next-intl"

import { Link } from "@/i18n/navigation"
import Footer from "@/components/Footer"
import LandingHeader from "./LandingHeader"
import ConnectionMotif from "./ConnectionMotif"

type IconProps = { className?: string }

function ShieldIcon({ className }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden="true">
      <path d="M12 3l7 3v5c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6l7-3z" />
      <path d="M9.5 12l1.8 1.8L15 10" />
    </svg>
  )
}

function KeyIcon({ className }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden="true">
      <circle cx="8" cy="8" r="4" />
      <path d="M11 11l8 8M16 16l2-2M19 19l2-2" />
    </svg>
  )
}

function ChatIcon({ className }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden="true">
      <path d="M4 5h16v11H9l-4 3v-3H4z" />
      <path d="M8 9h8M8 12h5" />
    </svg>
  )
}

function CheckIcon({ className }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden="true">
      <path d="M5 12.5l4.5 4.5L19 7" />
    </svg>
  )
}

function RoomsIcon({ className }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden="true">
      <path d="M4 9h16M4 15h16M9 4l-1.5 16M16.5 4L15 20" />
    </svg>
  )
}

const PRIMARY_CTA =
  "inline-flex items-center justify-center rounded-full bg-brand-accent px-7 py-3.5 text-base font-bold text-white shadow-[0_8px_30px_-8px_var(--brand-accent)] transition-colors hover:bg-brand-accent/85 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background"

const SECONDARY_CTA =
  "inline-flex items-center justify-center rounded-full px-7 py-3.5 text-base font-semibold text-foreground ring-1 ring-gray-700 transition-colors hover:bg-gray-800/40 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover"

export default function Landing() {
  const t = useTranslations("landing")
  const tNav = useTranslations("nav")

  const pillars = [
    { Icon: ShieldIcon, title: t("pillar1Title"), body: t("pillar1Body") },
    { Icon: KeyIcon, title: t("pillar2Title"), body: t("pillar2Body") },
    { Icon: ChatIcon, title: t("pillar3Title"), body: t("pillar3Body") },
  ]

  const steps = [
    { n: "01", title: t("step1Title"), body: t("step1Body") },
    { n: "02", title: t("step2Title"), body: t("step2Body") },
    { n: "03", title: t("step3Title"), body: t("step3Body") },
  ]

  const privacyPoints = [t("privacyPoint1"), t("privacyPoint2"), t("privacyPoint3"), t("privacyPoint4")]

  return (
    <div className="min-h-dvh bg-background text-foreground">
      <a
        href="#landing-main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[100] focus:rounded-md focus:bg-brand-primary focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-white focus:shadow-lg focus:outline-none focus:ring-2 focus:ring-white"
      >
        {tNav("skipToContent")}
      </a>

      <LandingHeader />

      <main id="landing-main">
        {/* ── Hero ─────────────────────────────────────────────── */}
        <section className="relative overflow-hidden">
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 -z-10"
            style={{
              background:
                "radial-gradient(60rem 40rem at 75% -10%, color-mix(in oklch, var(--brand-primary) 22%, transparent), transparent 70%), radial-gradient(50rem 36rem at 10% 110%, color-mix(in oklch, var(--brand-accent) 16%, transparent), transparent 70%)",
            }}
          />
          <div className="mx-auto grid max-w-6xl items-center gap-12 px-4 pb-16 pt-12 sm:px-6 lg:grid-cols-[1.05fr_0.95fr] lg:gap-8 lg:pb-24 lg:pt-20">
            <div>
              <h1 className="landing-rise font-display text-[clamp(2.5rem,6vw,4.25rem)] font-extrabold leading-[1.05] tracking-tight">
                {t("heroTitle")}
              </h1>

              <p
                className="landing-rise mt-6 max-w-xl text-lg leading-relaxed text-gray-400"
                style={{ animationDelay: "0.16s" }}
              >
                {t("heroSubtitle")}
              </p>

              <div
                className="landing-rise mt-9 flex flex-col gap-3 sm:flex-row sm:items-center"
                style={{ animationDelay: "0.24s" }}
              >
                <Link href="/register" className={PRIMARY_CTA}>
                  {t("ctaPrimary")}
                </Link>
                <Link href="/rooms" className={SECONDARY_CTA}>
                  {t("ctaSecondary")}
                </Link>
              </div>

              <p
                className="landing-rise mt-5 text-sm text-gray-500"
                style={{ animationDelay: "0.3s" }}
              >
                {t("ageNote")}{" "}
                <Link href="/login" className="font-semibold text-brand-muted underline-offset-2 hover:underline">
                  {t("ctaLogin")}
                </Link>
              </p>
            </div>

            <div className="landing-rise" style={{ animationDelay: "0.2s" }}>
              <ConnectionMotif />
            </div>
          </div>
        </section>

        {/* ── Value pillars ────────────────────────────────────── */}
        <section className="mx-auto max-w-6xl px-4 py-12 sm:px-6 lg:py-16">
          <div className="grid gap-px overflow-hidden rounded-card bg-gray-800/60 ring-1 ring-gray-800/60 sm:grid-cols-3">
            {pillars.map(({ Icon, title, body }) => (
              <div key={title} className="bg-background p-7">
                <Icon className="h-9 w-9 text-brand-accent" />
                <h3 className="mt-5 font-display text-xl font-bold">{title}</h3>
                <p className="mt-2.5 text-[0.95rem] leading-relaxed text-gray-400">{body}</p>
              </div>
            ))}
          </div>
        </section>

        {/* ── How it works ─────────────────────────────────────── */}
        <section className="mx-auto max-w-6xl px-4 py-12 sm:px-6 lg:py-20">
          <p className="text-sm font-bold uppercase tracking-widest text-brand-muted">{t("stepsEyebrow")}</p>
          <h2 className="mt-3 max-w-2xl font-display text-[clamp(1.875rem,4vw,2.75rem)] font-extrabold leading-tight tracking-tight">
            {t("stepsTitle")}
          </h2>

          <ol className="mt-10 grid gap-8 sm:grid-cols-3 lg:gap-12">
            {steps.map(({ n, title, body }) => (
              <li key={n} className="border-t-2 border-brand-accent/40 pt-5">
                <span className="font-display text-5xl font-extrabold text-brand-accent/30">{n}</span>
                <h3 className="mt-3 font-display text-xl font-bold">{title}</h3>
                <p className="mt-2 text-[0.95rem] leading-relaxed text-gray-400">{body}</p>
              </li>
            ))}
          </ol>
        </section>

        {/* ── Privacy (the differentiator, drenched panel) ─────── */}
        <section className="mx-auto max-w-6xl px-4 py-12 sm:px-6 lg:py-16">
          <div
            className="relative overflow-hidden rounded-[1.75rem] p-8 sm:p-12 lg:p-16"
            style={{ background: "linear-gradient(135deg, #0F3A60 0%, #1A5A8A 48%, #08203E 100%)" }}
          >
            <div
              aria-hidden="true"
              className="pointer-events-none absolute -right-16 -top-16 h-72 w-72 rounded-full blur-3xl"
              style={{ background: "radial-gradient(circle, var(--brand-accent), transparent 70%)", opacity: 0.4 }}
            />
            <div className="relative grid gap-10 lg:grid-cols-2 lg:gap-16">
              <div>
                <p className="text-sm font-bold uppercase tracking-widest text-white/60">{t("privacyEyebrow")}</p>
                <h2 className="mt-3 font-display text-[clamp(1.875rem,4vw,3rem)] font-extrabold leading-tight tracking-tight text-white">
                  {t("privacyTitle")}
                </h2>
                <p className="mt-5 max-w-md text-lg leading-relaxed text-white/70">{t("privacyBody")}</p>
              </div>
              <ul className="space-y-4 self-center">
                {privacyPoints.map((point) => (
                  <li key={point} className="flex items-start gap-3.5">
                    <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-white/15 text-white ring-1 ring-white/25">
                      <CheckIcon className="h-4 w-4" />
                    </span>
                    <span className="text-[1.0625rem] leading-relaxed text-white/90">{point}</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </section>

        {/* ── Public rooms teaser ──────────────────────────────── */}
        <section className="mx-auto max-w-6xl px-4 py-12 sm:px-6 lg:py-16">
          <div className="flex flex-col items-start gap-7 rounded-card bg-brand-surface-elevated p-8 ring-1 ring-brand-light/50 sm:p-10 lg:flex-row lg:items-center lg:justify-between">
            <div className="max-w-xl">
              <span className="inline-flex items-center gap-2 text-sm font-bold uppercase tracking-widest text-brand-muted">
                <RoomsIcon className="h-5 w-5" />
                {t("roomsEyebrow")}
              </span>
              <h2 className="mt-3 font-display text-2xl font-bold tracking-tight sm:text-3xl">{t("roomsTitle")}</h2>
              <p className="mt-3 text-[1.0625rem] leading-relaxed text-gray-400">{t("roomsBody")}</p>
            </div>
            <Link href="/rooms" className={`${SECONDARY_CTA} shrink-0`}>
              {t("roomsCta")}
            </Link>
          </div>
        </section>

        {/* ── Final CTA ────────────────────────────────────────── */}
        <section className="relative overflow-hidden px-4 py-20 text-center sm:px-6 lg:py-28">
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 -z-10"
            style={{
              background:
                "radial-gradient(48rem 30rem at 50% 50%, color-mix(in oklch, var(--brand-accent) 18%, transparent), transparent 70%)",
            }}
          />
          <h2 className="mx-auto max-w-3xl font-display text-[clamp(2rem,5vw,3.5rem)] font-extrabold leading-tight tracking-tight">
            {t("finalTitle")}
          </h2>
          <p className="mx-auto mt-5 max-w-xl text-lg text-gray-400">{t("finalBody")}</p>
          <div className="mt-9">
            <Link href="/register" className={PRIMARY_CTA}>
              {t("finalCta")}
            </Link>
          </div>
        </section>
      </main>

      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <Footer />
      </div>
    </div>
  )
}
