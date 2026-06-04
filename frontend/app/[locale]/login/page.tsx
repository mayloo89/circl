"use client"

import { signIn } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useState } from "react"
import { Link, useRouter } from "@/i18n/navigation"

import { loginSchema } from "@/lib/validation"
import PasswordField from "@/components/ui/PasswordField"
import Footer from "@/components/Footer"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function LoginPage() {
  const t = useTranslations("login")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [rememberMe, setRememberMe] = useState(false)
  const [error, setError] = useState("")
  const [emailNotVerified, setEmailNotVerified] = useState(false)
  const [resendSent, setResendSent] = useState(false)
  const [reactivated, setReactivated] = useState(false)
  const router = useRouter()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    setEmailNotVerified(false)
    setReactivated(false)

    const parsed = loginSchema.safeParse({ email, password })
    if (!parsed.success) {
      setError(parsed.error.issues[0].message)
      return
    }

    let wasReactivated = false
    try {
      const probe = await fetch(`${API_URL}/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: parsed.data.email, password: parsed.data.password }),
      })
      if (probe.status === 429) {
        const data = await probe.json().catch(() => ({}))
        const msg = ((data as { error?: string }).error ?? "").toLowerCase()
        if (msg.includes("account temporarily locked")) {
          setError(t("errorAccountLocked"))
        } else {
          setError(t("errorRateLimited"))
        }
        return
      }
      if (probe.ok) {
        const data = await probe.json() as { reactivated?: boolean }
        wasReactivated = data.reactivated === true
      }
    } catch { /* backend unreachable — let signIn handle the error */ }

    const result = await signIn("credentials", {
      email: parsed.data.email,
      password: parsed.data.password,
      rememberMe: String(rememberMe),
      redirect: false,
    })

    if (result?.error === "AccountLocked") {
      setError(t("errorAccountLocked"))
    } else if (result?.error === "RateLimited") {
      setError(t("errorRateLimited"))
    } else if (result?.error === "EmailNotVerified") {
      setEmailNotVerified(true)
    } else if (result?.error) {
      setError(t("errorInvalidCredentials"))
    } else if (wasReactivated) {
      setReactivated(true)
    } else {
      router.push("/")
    }
  }

  const handleResend = async () => {
    setResendSent(false)
    await fetch(`${API_URL}/auth/resend-verification`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    })
    setResendSent(true)
  }

  return (
    <div className="flex min-h-screen flex-col bg-gray-950">

      {reactivated && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" role="dialog" aria-modal="true">
          <div className="w-full max-w-sm rounded-xl bg-gray-900 p-8 text-center shadow-2xl ring-1 ring-green-800">
            <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-green-900 ring-1 ring-green-700">
              <svg className="h-7 w-7 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
            </div>
            <h2 className="text-xl font-bold text-foreground">{t("accountReactivated")}</h2>
            <p className="mt-2 text-sm text-gray-400">{t("accountReactivatedDesc")}</p>
            <button
              type="button"
              onClick={() => router.push("/")}
              className="mt-6 w-full rounded-md bg-green-600 px-4 py-2 text-sm font-semibold text-white hover:bg-green-500 focus:outline-none focus:ring-2 focus:ring-green-500 focus:ring-offset-2 focus:ring-offset-gray-900"
            >
              {t("goToHome")}
            </button>
          </div>
        </div>
      )}

      <main className="flex flex-1 items-center justify-center px-4 py-8">
      <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h2 className="text-center text-3xl font-bold text-foreground">Circl</h2>
          <p className="mt-2 text-center text-sm text-gray-400">{t("title")}</p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          {error && (
            <div className="rounded-md bg-red-950 p-4 text-sm text-red-400 ring-1 ring-red-900" role="alert">
              {error}
            </div>
          )}

          {emailNotVerified && (
            <div className="rounded-md bg-yellow-950 p-4 text-sm text-yellow-300 ring-1 ring-yellow-800" role="alert">
              <p className="font-medium">{t("emailNotVerified")}</p>
              <p className="mt-1 text-yellow-400">{t("emailNotVerifiedDesc")}</p>
              {resendSent ? (
                <p className="mt-2 text-green-400">{t("verificationSent")}</p>
              ) : (
                <button
                  type="button"
                  onClick={handleResend}
                  className="mt-2 text-yellow-300 underline hover:text-yellow-200"
                >
                  {t("resendVerification")}
                </button>
              )}
            </div>
          )}

          <div className="space-y-4">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-300">
                {t("email")}
              </label>
              <input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-foreground placeholder-gray-500 shadow-sm focus:border-brand-hover focus:outline-none focus:ring-brand-hover"
              />
            </div>
            <div>
              <PasswordField
                id="password"
                label={t("password")}
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
              />
              <div className="mt-1 text-right">
                <Link href="/forgot-password" className="text-xs text-brand-muted hover:text-brand-subtle">
                  {t("forgotPassword")}
                </Link>
              </div>
            </div>
          </div>

          <label className="flex items-center gap-2 text-sm text-gray-300 select-none cursor-pointer">
            <input
              type="checkbox"
              checked={rememberMe}
              onChange={(e) => setRememberMe(e.target.checked)}
              className="h-4 w-4 rounded border-gray-600 bg-gray-800 text-brand-hover focus:ring-brand-hover focus:ring-offset-gray-900"
            />
            {t("rememberMe")}
          </label>

          <button
            type="submit"
            className="w-full rounded-md bg-brand-primary px-4 py-2 text-foreground hover:bg-brand-hover focus:outline-none focus:ring-2 focus:ring-brand-hover focus:ring-offset-2 focus:ring-offset-gray-900"
          >
            {t("submit")}
          </button>
        </form>

        <p className="text-center text-sm text-gray-400">
          {t("noAccount")}{" "}
          <Link href="/register" className="font-medium text-brand-muted hover:text-brand-subtle">
            {t("createOne")}
          </Link>
        </p>

        <div className="border-t border-gray-800 pt-4 text-center">
          <Link href="/rooms" className="text-sm font-medium text-brand-primary hover:text-brand-hover">
            {t("browseAsGuest")}
          </Link>
        </div>
      </div>
      </main>
      <Footer />
    </div>
  )
}
