"use client"

import { Suspense, useState } from "react"
import { useSearchParams } from "next/navigation"
import { useTranslations } from "next-intl"

import { Link, useRouter } from "@/i18n/navigation"
import PasswordRequirements, { PASSWORD_RULES } from "@/components/ui/PasswordRequirements"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

function ResetPasswordForm() {
  const t = useTranslations("resetPassword")
  const searchParams = useSearchParams()
  const token = searchParams.get("token") ?? ""
  const router = useRouter()

  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  const allRulesMet = PASSWORD_RULES.every(({ test }) => test(password))
  const confirmMatch = !confirm || confirm === password

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")

    if (!token) {
      setError(t("invalidToken"))
      return
    }
    if (password !== confirm) {
      setError(t("passwordsMismatch"))
      return
    }
    if (!allRulesMet) {
      setError(t("doesNotMeetRequirements"))
      return
    }

    setLoading(true)
    const res = await fetch(`${API_URL}/auth/reset-password`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token, password }),
    })
    setLoading(false)

    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      setError(
        data.code === "invalid_token"
          ? t("expiredToken")
          : t("genericError"),
      )
      return
    }

    router.push("/login?reset=1")
  }

  if (!token) {
    return (
      <div className="text-center space-y-4">
        <p className="text-sm text-red-400">{t("invalidToken")}</p>
        <Link href="/forgot-password" className="text-sm text-blue-400 hover:text-blue-300">
          {t("requestNew")}
        </Link>
      </div>
    )
  }

  return (
    <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
      {error && (
        <div className="rounded-md bg-red-950 p-4 text-sm text-red-400 ring-1 ring-red-900" role="alert">
          {error}
        </div>
      )}
      <div className="space-y-4">
        <div>
          <label htmlFor="password" className="block text-sm font-medium text-gray-300">
            {t("newPassword")}
          </label>
          <input
            id="password"
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          />
          {password && <div className="mt-2"><PasswordRequirements password={password} /></div>}
        </div>
        <div>
          <label htmlFor="confirm" className="block text-sm font-medium text-gray-300">
            {t("confirmPassword")}
          </label>
          <input
            id="confirm"
            type="password"
            required
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          />
          {!confirmMatch && (
            <p className="mt-1 text-xs text-red-400" role="alert">{t("passwordsMismatch")}</p>
          )}
        </div>
      </div>
      <button
        type="submit"
        disabled={loading || !allRulesMet || !confirmMatch}
        className="w-full rounded-md bg-blue-600 px-4 py-2 text-white hover:bg-blue-500 disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-gray-900"
      >
        {loading ? t("submitting") : t("submit")}
      </button>
    </form>
  )
}

export default function ResetPasswordPage() {
  const t = useTranslations("resetPassword")
  const tc = useTranslations("common")

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-950">
      <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h2 className="text-center text-3xl font-bold text-white">{t("title")}</h2>
          <p className="mt-2 text-center text-sm text-gray-400">{t("subtitle")}</p>
        </div>
        <Suspense fallback={<p className="text-center text-sm text-gray-400">{tc("loading")}</p>}>
          <ResetPasswordForm />
        </Suspense>
      </div>
    </div>
  )
}
