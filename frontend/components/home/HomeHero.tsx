"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useTheme } from "@/contexts/ThemeContext"

export default function HomeHero() {
  const t = useTranslations("home")
  const { data: session } = useSession()
  const { theme } = useTheme()
  const firstName = session?.user?.name?.split(" ")[0] ?? null

  return (
    <div className="px-4 py-5 text-center lg:py-7">
      <h1>
        <Image
          src={theme === "dark" ? "/branding/logo-dark.svg" : "/branding/logo-light.svg"}
          alt="circl"
          width={85}
          height={48}
          priority
          className="mx-auto h-12 w-auto"
        />
      </h1>

      {/* Tagline */}
      <p className="mt-2 text-sm text-gray-500">{t("tagline")}</p>

      {/* Personalised greeting — only when session is loaded */}
      {firstName && (
        <p className="mt-3 text-base font-semibold text-foreground">
          {t("hello", { name: firstName })}
        </p>
      )}
    </div>
  )
}
