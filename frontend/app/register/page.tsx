"use client"

import { signIn } from "next-auth/react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useState } from "react"

import { registerSchema } from "@/lib/validation"
import PasswordRequirements, { PASSWORD_RULES } from "@/components/ui/PasswordRequirements"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

async function seedProfile(token: string, username: string, dateOfBirth: string, displayName: string) {
  return fetch(`${API_URL}/profiles/me`, {
    method: "PUT",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify({ username, date_of_birth: dateOfBirth, display_name: displayName, interests: [] }),
  })
}

type FieldErrors = {
  email?: string
  username?: string
  date_of_birth?: string
  password?: string
  confirm?: string
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
  const router = useRouter()

  const [retryToken, setRetryToken] = useState<string | null>(null)
  const [retryUsername, setRetryUsername] = useState("")

  const allRulesMet = PASSWORD_RULES.every(({ test }) => test(password))

  function validateField(field: keyof FieldErrors, value: string) {
    const shape = registerSchema.shape as Record<string, { safeParse: (v: unknown) => { success: boolean; error?: { issues: { message: string }[] } } }>
    if (field === "confirm") {
      if (value && value !== password) {
        setFieldErrors((prev) => ({ ...prev, confirm: "Passwords do not match" }))
      } else {
        setFieldErrors((prev) => ({ ...prev, confirm: undefined }))
      }
      return
    }
    const fieldSchema = shape[field]
    if (!fieldSchema) return
    const result = fieldSchema.safeParse(value)
    setFieldErrors((prev) => ({
      ...prev,
      [field]: result.success ? undefined : result.error?.issues[0]?.message,
    }))
  }

  async function finishWithToken(token: string, takenUsername: string) {
    setRetryToken(token)
    setRetryUsername(takenUsername)
  }

  async function signInAndRedirect(destination: string) {
    const result = await signIn("credentials", { email, password, redirect: false })
    if (result?.ok) {
      router.push(destination)
    } else {
      router.push("/login")
    }
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
        body: JSON.stringify({ email: parsed.data.email, password: parsed.data.password }),
      })

      if (res.status === 409) { setSubmitError("An account with this email already exists"); return }
      if (res.status === 429) { setSubmitError("Too many registrations from this network. Please try again later."); return }
      if (res.status === 400) { const body = await res.json(); setSubmitError(body.error ?? "Invalid input"); return }
      if (!res.ok) { setSubmitError("Registration failed. Please try again."); return }

      const { token } = await res.json()
      const displayName = parsed.data.email.split("@")[0]
      const profileRes = await seedProfile(token, parsed.data.username, parsed.data.date_of_birth, displayName)

      if (profileRes.status === 409) {
        await finishWithToken(token, parsed.data.username)
        return
      }

      await signInAndRedirect(profileRes.ok ? "/" : "/profile")
    } catch {
      setSubmitError("Network error. Please try again.")
    } finally {
      setLoading(false)
    }
  }

  async function handleRetryUsername(e: React.FormEvent) {
    e.preventDefault()
    if (!retryToken) return

    if (!/^[a-z0-9_]{3,30}$/.test(retryUsername)) {
      setSubmitError("Username must be 3–30 characters: lowercase letters, digits, or underscores")
      return
    }

    setLoading(true)
    setSubmitError("")
    try {
      const displayName = email.split("@")[0]
      const profileRes = await seedProfile(retryToken, retryUsername, dateOfBirth, displayName)

      if (profileRes.status === 409) {
        setSubmitError("Username already taken. Please choose a different one.")
        return
      }

      await signInAndRedirect(profileRes.ok ? "/" : "/profile")
    } catch {
      setSubmitError("Network error. Please try again.")
    } finally {
      setLoading(false)
    }
  }

  if (retryToken) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950">
        <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
          <div>
            <h2 className="text-center text-3xl font-bold text-white">Circl</h2>
            <p className="mt-2 text-center text-sm text-gray-400">Choose a username</p>
          </div>

          <form className="mt-8 space-y-6" onSubmit={handleRetryUsername}>
            {submitError && (
              <div className="rounded-md bg-red-950 p-4 text-sm text-red-400 ring-1 ring-red-900">{submitError}</div>
            )}

            <p className="rounded-lg bg-amber-950 p-3 text-sm text-amber-300 ring-1 ring-amber-700">
              Your account was created, but <strong>@{username}</strong> is already taken. Please choose a different username.
            </p>

            <div>
              <label htmlFor="retry-username" className="block text-sm font-medium text-gray-300">Username</label>
              <div className="relative mt-1">
                <span className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-gray-500">@</span>
                <input
                  id="retry-username"
                  type="text"
                  required
                  value={retryUsername}
                  onChange={(e) => setRetryUsername(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ""))}
                  maxLength={30}
                  placeholder="your_username"
                  className="block w-full rounded-md border border-gray-700 bg-gray-800 pl-7 pr-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full rounded-md bg-indigo-600 px-4 py-2 text-white hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900 disabled:opacity-50"
            >
              {loading ? "Continuing…" : "Continue"}
            </button>
          </form>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-950">
      <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h2 className="text-center text-3xl font-bold text-white">Circl</h2>
          <p className="mt-2 text-center text-sm text-gray-400">Create your account</p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit} noValidate>
          {submitError && (
            <div className="rounded-md bg-red-950 p-4 text-sm text-red-400 ring-1 ring-red-900">{submitError}</div>
          )}

          <div className="space-y-4">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-300">Email</label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onBlur={() => validateField("email", email)}
                className={fieldClass(fieldErrors.email)}
              />
              {fieldErrors.email && <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.email}</p>}
            </div>

            <div>
              <label htmlFor="username" className="block text-sm font-medium text-gray-300">
                Username <span className="font-normal text-gray-500">(3–30 chars, lowercase, digits, _)</span>
              </label>
              <div className="relative mt-1">
                <span className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-gray-500">@</span>
                <input
                  id="username"
                  type="text"
                  autoComplete="username"
                  required
                  value={username}
                  onChange={(e) => setUsername(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ""))}
                  onBlur={() => validateField("username", username)}
                  maxLength={30}
                  placeholder="your_username"
                  className={`block w-full rounded-md border ${fieldErrors.username ? "border-red-500" : "border-gray-700"} bg-gray-800 pl-7 pr-3 py-2 text-white placeholder-gray-500 shadow-sm focus:outline-none focus:ring-1 ${fieldErrors.username ? "focus:border-red-400 focus:ring-red-400" : "focus:border-indigo-500 focus:ring-indigo-500"}`}
                />
              </div>
              {fieldErrors.username && <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.username}</p>}
            </div>

            <div>
              <label htmlFor="date-of-birth" className="block text-sm font-medium text-gray-300">
                Date of birth <span className="font-normal text-gray-500">(must be 18+)</span>
              </label>
              <input
                id="date-of-birth"
                type="date"
                required
                value={dateOfBirth}
                onChange={(e) => setDateOfBirth(e.target.value)}
                onBlur={() => validateField("date_of_birth", dateOfBirth)}
                className={fieldClass(fieldErrors.date_of_birth)}
              />
              {fieldErrors.date_of_birth && <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.date_of_birth}</p>}
            </div>

            <div className="flex flex-col gap-2">
              <div>
                <label htmlFor="password" className="block text-sm font-medium text-gray-300">Password</label>
                <input
                  id="password"
                  type="password"
                  autoComplete="new-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  onBlur={() => validateField("password", password)}
                  className={fieldClass(fieldErrors.password)}
                />
                {fieldErrors.password && !password && (
                  <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.password}</p>
                )}
              </div>
              <PasswordRequirements password={password} />
            </div>

            <div>
              <label htmlFor="confirm" className="block text-sm font-medium text-gray-300">Confirm password</label>
              <input
                id="confirm"
                type="password"
                autoComplete="new-password"
                required
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                onBlur={() => validateField("confirm", confirm)}
                className={fieldClass(fieldErrors.confirm)}
              />
              {fieldErrors.confirm && <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.confirm}</p>}
            </div>
          </div>

          <button
            type="submit"
            disabled={loading || !allRulesMet}
            className="w-full rounded-md bg-indigo-600 px-4 py-2 text-white hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900 disabled:opacity-50"
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
