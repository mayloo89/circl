"use client"

import Link from "next/link"
import { useSearchParams } from "next/navigation"
import { Suspense, useEffect, useRef, useState } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

function VerifyEmailContent() {
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
          data.error === "invalid or expired verification token"
            ? "This verification link has expired or already been used."
            : "Something went wrong. Please try again."
        )
        setStatus("error")
      }
    }

    verify()
  }, [token])

  if (!token) {
    return (
      <div className="text-center space-y-4">
        <p className="text-sm text-red-400">Invalid verification link.</p>
        <Link href="/login" className="text-sm text-blue-400 hover:text-blue-300">
          Back to sign in
        </Link>
      </div>
    )
  }

  if (status === "pending") {
    return <p className="text-center text-sm text-gray-400">Verifying your email…</p>
  }

  if (status === "error") {
    return (
      <div className="text-center space-y-4">
        <p className="text-sm text-red-400">{errorMessage}</p>
        <Link href="/login" className="block text-sm text-blue-400 hover:text-blue-300">
          Back to sign in
        </Link>
      </div>
    )
  }

  return (
    <div className="text-center space-y-4">
      <div className="text-green-400 text-5xl">✓</div>
      <p className="text-sm text-gray-300">Your email has been verified. You can now sign in.</p>
      <Link
        href="/login"
        className="inline-block rounded-md bg-blue-600 px-6 py-2 text-sm font-medium text-white hover:bg-blue-500"
      >
        Sign in
      </Link>
    </div>
  )
}

export default function VerifyEmailPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-950">
      <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h2 className="text-center text-3xl font-bold text-white">Email verification</h2>
        </div>
        <Suspense fallback={<p className="text-center text-sm text-gray-400">Loading…</p>}>
          <VerifyEmailContent />
        </Suspense>
      </div>
    </div>
  )
}
