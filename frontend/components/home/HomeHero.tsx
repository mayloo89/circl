"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"

export default function HomeHero() {
  const t = useTranslations("home")
  const { data: session } = useSession()
  const firstName = session?.user?.name?.split(" ")[0] ?? null

  return (
    <div className="relative overflow-hidden px-4 py-10 text-center lg:px-0 lg:py-12">
      {/* Ambient blobs — visible in dark, very subtle in light */}
      <div aria-hidden="true" className="pointer-events-none absolute inset-0">
        <div className="absolute -top-24 -left-16 h-72 w-72 rounded-full bg-brand-primary/10 blur-3xl" />
        <div className="absolute -top-8 -right-12 h-56 w-56 rounded-full bg-brand-accent/[0.07] blur-3xl" />
        <div className="absolute top-1/2 left-1/3 h-44 w-44 rounded-full bg-purple-500/[0.05] blur-3xl" />
      </div>

      {/* Logo mark with glow halo */}
      <div className="relative inline-flex items-center justify-center">
        <div
          aria-hidden="true"
          className="absolute h-20 w-20 rounded-full bg-brand-primary/20 blur-2xl"
        />
        <Image
          src="/branding/mark.svg"
          alt=""
          width={52}
          height={52}
          priority
          aria-hidden="true"
          className="relative"
        />
      </div>

      {/* Wordmark */}
      <h1 className="relative mt-3 font-display text-[2rem] font-extrabold leading-none text-foreground" style={{ letterSpacing: "-0.04em" }}>
        circl
      </h1>

      {/* Tagline */}
      <p className="relative mt-1.5 text-sm text-gray-500">{t("tagline")}</p>

      {/* Personalised greeting — only when session is loaded */}
      {firstName && (
        <p className="relative mt-4 text-sm font-medium text-gray-400">
          {t("hello", { name: firstName })}
        </p>
      )}
    </div>
  )
}
