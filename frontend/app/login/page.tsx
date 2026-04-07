"use client"

import { signIn } from "next-auth/react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useState } from "react"

import { loginSchema } from "@/lib/validation"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function LoginPage() {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [emailNotVerified, setEmailNotVerified] = useState(false)
  const [resendSent, setResendSent] = useState(false)
  const [accountDeleted, setAccountDeleted] = useState(false)
  const [reactivating, setReactivating] = useState(false)
  const [reactivateError, setReactivateError] = useState("")
  const router = useRouter()

  const resetAlerts = () => {
    setError("")
    setEmailNotVerified(false)
    setAccountDeleted(false)
    setReactivateError("")
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    resetAlerts()

    const parsed = loginSchema.safeParse({ email, password })
    if (!parsed.success) {
      setError(parsed.error.issues[0].message)
      return
    }

    const result = await signIn("credentials", {
      email: parsed.data.email,
      password: parsed.data.password,
      redirect: false,
    })

    if (result?.error === "AccountLocked") {
      setError("Account temporarily locked due to too many failed login attempts. Please try again in 15 minutes.")
    } else if (result?.error === "EmailNotVerified") {
      setEmailNotVerified(true)
    } else if (result?.error === "AccountDeleted") {
      setAccountDeleted(true)
    } else if (result?.error) {
      setError("Invalid email or password.")
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

  const handleReactivate = async () => {
    setReactivating(true)
    setReactivateError("")
    try {
      const res = await fetch(`${API_URL}/auth/reactivate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        setReactivateError((data as { error?: string }).error ?? "Reactivation failed. Please try again.")
        return
      }
      // Account reactivated — sign in normally
      const result = await signIn("credentials", { email, password, redirect: false })
      if (result?.error) {
        setReactivateError("Account reactivated but sign-in failed. Please try again.")
      } else {
        router.push("/")
      }
    } finally {
      setReactivating(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-950">
      <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h2 className="text-center text-3xl font-bold text-white">Circl</h2>
          <p className="mt-2 text-center text-sm text-gray-400">Sign in to your account</p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          {error && (
            <div className="rounded-md bg-red-950 p-4 text-sm text-red-400 ring-1 ring-red-900" role="alert">
              {error}
            </div>
          )}

          {emailNotVerified && (
            <div className="rounded-md bg-yellow-950 p-4 text-sm text-yellow-300 ring-1 ring-yellow-800" role="alert">
              <p className="font-medium">Email not verified</p>
              <p className="mt-1 text-yellow-400">Check your inbox for the verification link.</p>
              {resendSent ? (
                <p className="mt-2 text-green-400">Verification email sent — check your inbox.</p>
              ) : (
                <button
                  type="button"
                  onClick={handleResend}
                  className="mt-2 text-yellow-300 underline hover:text-yellow-200"
                >
                  Resend verification email
                </button>
              )}
            </div>
          )}

          {accountDeleted && (
            <div className="rounded-md bg-orange-950 p-4 text-sm text-orange-300 ring-1 ring-orange-900" role="alert">
              <p className="font-medium">Account scheduled for deletion</p>
              <p className="mt-1 text-orange-400">
                Your account is within the 30-day grace period. You can reactivate it now and nothing will be lost.
              </p>
              {reactivateError && (
                <p className="mt-2 text-red-400">{reactivateError}</p>
              )}
              <button
                type="button"
                onClick={handleReactivate}
                disabled={reactivating}
                className="mt-3 w-full rounded-md bg-orange-700 px-3 py-2 text-sm font-medium text-white hover:bg-orange-600 disabled:opacity-50"
              >
                {reactivating ? "Reactivating…" : "Reactivate my account"}
              </button>
            </div>
          )}

          <div className="space-y-4">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-300">
                Email
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
            <div>
              <div className="flex items-center justify-between">
                <label htmlFor="password" className="block text-sm font-medium text-gray-300">
                  Password
                </label>
                <Link href="/forgot-password" className="text-xs text-blue-400 hover:text-blue-300">
                  Forgot password?
                </Link>
              </div>
              <input
                id="password"
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
              />
            </div>
          </div>
          <button
            type="submit"
            className="w-full rounded-md bg-blue-600 px-4 py-2 text-white hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-gray-900"
          >
            Sign in
          </button>
        </form>

        <p className="text-center text-sm text-gray-400">
          Don&apos;t have an account?{" "}
          <Link href="/register" className="font-medium text-blue-400 hover:text-blue-300">
            Create one
          </Link>
        </p>
      </div>
    </div>
  )
}
