"use client"

import { Suspense, useEffect, useRef, useState } from "react"
import { useSearchParams } from "next/navigation"
import { useTranslations } from "next-intl"

import { Link } from "@/i18n/navigation"
import Footer from "@/components/Footer"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

function VerifyEmailContent() {
  const t = useTranslations("verifyEmail")
  const searchParams = useSearchParams()
  const token = searchParams.get("token") ?? ""

  const [status, setStatus] = useState<"pending" | "success" | "error">("pending")
  const [errorMessage, setErrorMessage] = useState("")
  const didVerify = useRef(false)

  useEffect(() => {
    if (!token || didVerify.current) return
    didVerify.current = true

    async function verify() {
      const res = await fetch(`${API_URL}/auth/verify-email`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      })

      if (res.ok) {
        setStatus("success")
      } else {
        const data = await res.json().catch(() => ({}))
        setErrorMessage(
          data.code === "invalid_token"
            ? t("expiredToken")
            : t("genericError"),
        )
        setStatus("error")
      }
    }

    verify()
  }, [token, t])

  if (!token) {
    return (
      <div className="text-center space-y-4">
        <p className="text-sm text-red-400">{t("invalidLink")}</p>
        <Link href="/login" className="text-sm text-brand-muted hover:text-brand-subtle">
          {t("backToSignIn")}
        </Link>
      </div>
    )
  }

  if (status === "pending") {
    return <p className="text-center text-sm text-gray-400">{t("verifying")}</p>
  }

  if (status === "error") {
    return (
      <div className="text-center space-y-4">
        <p className="text-sm text-red-400">{errorMessage}</p>
        <Link href="/login" className="block text-sm text-brand-muted hover:text-brand-subtle">
          {t("backToSignIn")}
        </Link>
      </div>
    )
  }

  return (
    <div className="text-center space-y-4">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-green-900/50 ring-1 ring-green-700">
        <svg className="h-7 w-7 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
        </svg>
      </div>
      <p className="text-sm text-gray-300">{t("success")}</p>
      <Link
        href="/login"
        className="inline-block rounded-md bg-brand-primary px-6 py-3 text-sm font-medium text-foreground hover:bg-brand-hover"
      >
        {t("signIn")}
      </Link>
    </div>
  )
}

export default function VerifyEmailPage() {
  const t = useTranslations("verifyEmail")
  const tc = useTranslations("common")

  return (
    <div className="flex min-h-screen flex-col bg-gray-950">
      <main className="flex flex-1 items-center justify-center px-4 py-8">
        <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
          <div>
            <h2 className="text-center text-3xl font-bold text-foreground">{t("title")}</h2>
          </div>
          <Suspense fallback={<p className="text-center text-sm text-gray-400">{tc("loading")}</p>}>
            <VerifyEmailContent />
          </Suspense>
        </div>
      </main>
      <Footer />
    </div>
  )
}
