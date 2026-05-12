"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"

import { Link } from "@/i18n/navigation"
import Footer from "@/components/Footer"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function ForgotPasswordPage() {
  const t = useTranslations("forgotPassword")
  const [email, setEmail] = useState("")
  const [submitted, setSubmitted] = useState(false)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    await fetch(`${API_URL}/auth/forgot-password`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    })
    setLoading(false)
    setSubmitted(true)
  }

  return (
    <div className="flex min-h-screen flex-col bg-gray-950">
      <main className="flex flex-1 items-center justify-center px-4 py-8">
      <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h2 className="text-center text-3xl font-bold text-white">{t("title")}</h2>
          <p className="mt-2 text-center text-sm text-gray-400">{t("subtitle")}</p>
        </div>

        {submitted ? (
          <div className="space-y-4 text-center">
            <p className="text-sm text-gray-300">{t("successMessage")}</p>
            <Link href="/login" className="block text-sm text-blue-400 hover:text-blue-300">
              {t("backToSignIn")}
            </Link>
          </div>
        ) : (
          <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
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
                className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
              />
            </div>
            <button
              type="submit"
              disabled={loading}
              className="w-full rounded-md bg-blue-600 px-4 py-2 text-white hover:bg-blue-500 disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-gray-900"
            >
              {loading ? t("submitting") : t("submit")}
            </button>
            <p className="text-center text-sm text-gray-400">
              <Link href="/login" className="text-blue-400 hover:text-blue-300">
                {t("backToSignIn")}
              </Link>
            </p>
          </form>
        )}
      </div>
      </main>
      <Footer />
    </div>
  )
}
