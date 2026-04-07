"use client"

import Link from "next/link"
import { useState } from "react"

import { registerSchema } from "@/lib/validation"
import PasswordRequirements, { PASSWORD_RULES } from "@/components/ui/PasswordRequirements"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

type FieldErrors = {
  email?: string
  password?: string
  confirm?: string
  username?: string
  date_of_birth?: string
}

function fieldClass(error?: string) {
  return `mt-1 block w-full rounded-md border ${error ? "border-red-500" : "border-gray-700"} bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:outline-none focus:ring-1 ${error ? "focus:border-red-400 focus:ring-red-400" : "focus:border-indigo-500 focus:ring-indigo-500"}`
}

export default function RegisterPage() {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [username, setUsername] = useState("")
  const [dateOfBirth, setDateOfBirth] = useState("")
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [submitError, setSubmitError] = useState("")
  const [loading, setLoading] = useState(false)
  const [registered, setRegistered] = useState(false)

  const allRulesMet = PASSWORD_RULES.every(({ test }) => test(password))

  function validateField(field: keyof FieldErrors, value: string) {
    if (field === "confirm") {
      setFieldErrors((prev) => ({
        ...prev,
        confirm: value && value !== password ? "Passwords do not match" : undefined,
      }))
      return
    }
    const shape = registerSchema.shape as Record<string, { safeParse: (v: unknown) => { success: boolean; error?: { issues: { message: string }[] } } }>
    const fieldSchema = shape[field]
    if (!fieldSchema) return
    const result = fieldSchema.safeParse(value)
    setFieldErrors((prev) => ({
      ...prev,
      [field]: result.success ? undefined : result.error?.issues[0]?.message,
    }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitError("")

    const parsed = registerSchema.safeParse({ email, password, confirm, username, date_of_birth: dateOfBirth })
    if (!parsed.success) {
      const errors: FieldErrors = {}
      for (const issue of parsed.error.issues) {
        const path = issue.path[0] as keyof FieldErrors
        if (!errors[path]) errors[path] = issue.message
      }
      setFieldErrors(errors)
      return
    }

    setLoading(true)
    try {
      const res = await fetch(`${API_URL}/auth/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: parsed.data.email,
          password: parsed.data.password,
          username: parsed.data.username,
          date_of_birth: parsed.data.date_of_birth,
        }),
      })

      if (res.status === 409) {
        const body = await res.json()
        if (body.error === "username already taken") {
          setFieldErrors((prev) => ({ ...prev, username: "Username already taken" }))
        } else {
          setSubmitError("An account with this email already exists.")
        }
        return
      }
      if (res.status === 429) { setSubmitError("Too many registrations from this network. Please try again later."); return }
      if (res.status === 400) {
        const body = await res.json()
        setSubmitError(body.error ?? "Invalid input.")
        return
      }
      if (!res.ok) { setSubmitError("Registration failed. Please try again."); return }

      setRegistered(true)
    } catch {
      setSubmitError("Network error. Please try again.")
    } finally {
      setLoading(false)
    }
  }

  if (registered) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950">
        <div className="w-full max-w-md space-y-6 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800 text-center">
          <div className="text-5xl text-indigo-400">✉</div>
          <h2 className="text-2xl font-bold text-white">Check your email</h2>
          <p className="text-sm text-gray-400">
            We sent a verification link to <span className="text-white font-medium">{email}</span>.
            Click it to activate your account before signing in.
          </p>
          <p className="text-xs text-gray-500">
            Didn&apos;t receive it?{" "}
            <button
              type="button"
              className="text-indigo-400 hover:text-indigo-300 underline"
              onClick={async () => {
                await fetch(`${API_URL}/auth/resend-verification`, {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ email }),
                })
              }}
            >
              Resend verification email
            </button>
          </p>
          <Link href="/login" className="block text-sm text-indigo-400 hover:text-indigo-300">
            Back to sign in
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-950">
      <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h2 className="text-center text-3xl font-bold text-white">Create account</h2>
          <p className="mt-2 text-center text-sm text-gray-400">Join Circl today</p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          {submitError && (
            <div className="rounded-md bg-red-950 p-4 text-sm text-red-400 ring-1 ring-red-900" role="alert">
              {submitError}
            </div>
          )}

          <div className="space-y-4">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-300">
                Email address
              </label>
              <input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onBlur={(e) => validateField("email", e.target.value)}
                className={fieldClass(fieldErrors.email)}
              />
              {fieldErrors.email && (
                <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.email}</p>
              )}
            </div>

            <div>
              <label htmlFor="username" className="block text-sm font-medium text-gray-300">
                Username
              </label>
              <input
                id="username"
                type="text"
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                onBlur={(e) => validateField("username", e.target.value)}
                className={fieldClass(fieldErrors.username)}
                placeholder="lowercase_letters_digits"
              />
              {fieldErrors.username && (
                <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.username}</p>
              )}
            </div>

            <div>
              <label htmlFor="date_of_birth" className="block text-sm font-medium text-gray-300">
                Date of birth
              </label>
              <input
                id="date_of_birth"
                type="date"
                required
                value={dateOfBirth}
                onChange={(e) => setDateOfBirth(e.target.value)}
                onBlur={(e) => validateField("date_of_birth", e.target.value)}
                className={fieldClass(fieldErrors.date_of_birth)}
              />
              {fieldErrors.date_of_birth && (
                <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.date_of_birth}</p>
              )}
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
                onBlur={(e) => validateField("password", e.target.value)}
                className={fieldClass(fieldErrors.password)}
              />
              {fieldErrors.password && (
                <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.password}</p>
              )}
              {password && (
                <div className="mt-2">
                  <PasswordRequirements password={password} />
                </div>
              )}
            </div>

            <div>
              <label htmlFor="confirm" className="block text-sm font-medium text-gray-300">
                Confirm password
              </label>
              <input
                id="confirm"
                type="password"
                required
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                onBlur={(e) => validateField("confirm", e.target.value)}
                className={fieldClass(fieldErrors.confirm)}
              />
              {fieldErrors.confirm && (
                <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.confirm}</p>
              )}
            </div>
          </div>

          <button
            type="submit"
            disabled={loading || !allRulesMet}
            className="w-full rounded-md bg-indigo-600 px-4 py-2 text-white hover:bg-indigo-500 disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900"
          >
            {loading ? "Creating account…" : "Create account"}
          </button>
        </form>

        <p className="text-center text-sm text-gray-400">
          Already have an account?{" "}
          <Link href="/login" className="font-medium text-indigo-400 hover:text-indigo-300">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  )
}
