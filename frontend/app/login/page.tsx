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

    // Call the backend directly first to capture the reactivated flag before
    // going through NextAuth, which doesn't expose extra response fields.
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
          setError("Account temporarily locked due to too many failed login attempts. Please try again in 15 minutes.")
        } else {
          setError("Too many login attempts from your network. Please wait a moment before trying again.")
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
      redirect: false,
    })

    if (result?.error === "AccountLocked") {
      setError("Account temporarily locked due to too many failed login attempts. Please try again in 15 minutes.")
    } else if (result?.error === "RateLimited") {
      setError("Too many login attempts from your network. Please wait a moment before trying again.")
    } else if (result?.error === "EmailNotVerified") {
      setEmailNotVerified(true)
    } else if (result?.error) {
      setError("Invalid email or password.")
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
    <div className="flex min-h-screen items-center justify-center bg-gray-950">

      {reactivated && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" role="dialog" aria-modal="true">
          <div className="w-full max-w-sm rounded-xl bg-gray-900 p-8 text-center shadow-2xl ring-1 ring-green-800">
            <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-green-900 ring-1 ring-green-700">
              <svg className="h-7 w-7 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
            </div>
            <h2 className="text-xl font-bold text-white">Account reactivated</h2>
            <p className="mt-2 text-sm text-gray-400">
              Welcome back! Your account has been restored. All your data is intact.
            </p>
            <button
              type="button"
              onClick={() => router.push("/")}
              className="mt-6 w-full rounded-md bg-green-600 px-4 py-2 text-sm font-semibold text-white hover:bg-green-500 focus:outline-none focus:ring-2 focus:ring-green-500 focus:ring-offset-2 focus:ring-offset-gray-900"
            >
              Go to home
            </button>
          </div>
        </div>
      )}

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
              <label htmlFor="password" className="block text-sm font-medium text-gray-300">
                Password
              </label>
              <input
                id="password"
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
              />
              <div className="mt-1 text-right">
                <Link href="/forgot-password" className="text-xs text-blue-400 hover:text-blue-300">
                  Forgot password?
                </Link>
              </div>
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
