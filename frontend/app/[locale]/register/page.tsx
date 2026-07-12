"use client"

import { useTranslations } from "next-intl"
import { useEffect, useRef, useState } from "react"
import { Link } from "@/i18n/navigation"

import { registerSchema } from "@/lib/validation"
import PasswordRequirements, { PASSWORD_RULES } from "@/components/ui/PasswordRequirements"
import PasswordField from "@/components/ui/PasswordField"
import DateOfBirthPicker from "@/components/ui/DateOfBirthPicker"
import Checkbox from "@/components/ui/Checkbox"
import Footer from "@/components/Footer"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

type FieldErrors = {
  email?: string
  password?: string
  confirm?: string
  username?: string
  date_of_birth?: string
}

function fieldClass(error?: string) {
  return `mt-1 block w-full rounded-md border ${error ? "border-red-500" : "border-gray-700"} bg-gray-800 px-3 py-2 text-foreground placeholder-gray-500 shadow-sm focus:outline-none focus:ring-1 ${error ? "focus:border-red-400 focus:ring-red-400" : "focus:border-brand-hover focus:ring-brand-hover"}`
}

export default function RegisterPage() {
  const t = useTranslations("register")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [username, setUsername] = useState("")
  const [dateOfBirth, setDateOfBirth] = useState("")
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [touched, setTouched] = useState<Partial<Record<keyof FieldErrors, boolean>>>({})
  const [passwordFocused, setPasswordFocused] = useState(false)
  const [submitError, setSubmitError] = useState("")
  const [loading, setLoading] = useState(false)
  const [registered, setRegistered] = useState(false)

  const [acceptTerms, setAcceptTerms] = useState(false)
  const [consentError, setConsentError] = useState<string | undefined>(undefined)
  const consentRef = useRef<HTMLInputElement>(null)

  const [usernameAvailable, setUsernameAvailable] = useState<boolean | null>(null)
  const [usernameChecking, setUsernameChecking] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    const usernameRegex = /^[a-z0-9_]{3,30}$/
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(async () => {
      if (!usernameRegex.test(username)) {
        setUsernameAvailable(null)
        setUsernameChecking(false)
        return
      }
      setUsernameChecking(true)
      try {
        const res = await fetch(`${API_URL}/profiles/available?username=${encodeURIComponent(username)}`)
        if (res.ok) {
          const data = await res.json()
          setUsernameAvailable(data.available)
        }
      } catch {
        // silently ignore — availability check is best-effort
      } finally {
        setUsernameChecking(false)
      }
    }, 400)
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current) }
  }, [username])

  const allRulesMet = PASSWORD_RULES.every(({ test }) => test(password))

  function validateField(field: keyof FieldErrors, value: string) {
    if (field === "confirm") {
      setFieldErrors((prev) => ({
        ...prev,
        confirm: value && value !== password ? t("passwordsMismatch") : undefined,
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

  function touchField(field: keyof FieldErrors) {
    setTouched((prev) => ({ ...prev, [field]: true }))
  }

  function handleChange(field: keyof FieldErrors, value: string) {
    if (field === "email") setEmail(value)
    else if (field === "password") setPassword(value)
    else if (field === "confirm") setConfirm(value)
    else if (field === "username") { setUsername(value); setUsernameAvailable(null) }
    else if (field === "date_of_birth") setDateOfBirth(value)
    if (touched[field]) validateField(field, value)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitError("")
    setConsentError(undefined)

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

    if (!acceptTerms) {
      setConsentError(t("consentRequired"))
      consentRef.current?.focus()
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
          accept_terms: true,
        }),
      })

      if (res.status === 409) {
        const body = await res.json()
        if (body.code === "username_taken") {
          setFieldErrors((prev) => ({ ...prev, username: t("usernameTaken") }))
        } else {
          setSubmitError(t("emailTaken"))
        }
        return
      }
      if (res.status === 429) { setSubmitError(t("rateLimited")); return }
      if (res.status === 400) {
        const body = await res.json()
        if (body.code === "terms_not_accepted") {
          setConsentError(t("consentRequired"))
          consentRef.current?.focus()
          return
        }
        setSubmitError(body.error ?? t("invalidInput"))
        return
      }
      if (!res.ok) { setSubmitError(t("failed")); return }

      setRegistered(true)
    } catch {
      setSubmitError(t("failed"))
    } finally {
      setLoading(false)
    }
  }

  if (registered) {
    return (
      <div className="flex min-h-screen flex-col bg-gray-950">
        <main className="flex flex-1 items-center justify-center px-4 py-8">
        <div className="w-full max-w-md space-y-6 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800 text-center">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-brand-wash/50 ring-1 ring-brand-strong/60">
            <svg className="h-8 w-8 text-brand-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5} aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75" />
            </svg>
          </div>
          <h2 className="text-2xl font-bold text-foreground">{t("checkEmail")}</h2>
          <p className="text-sm text-gray-400">
            {t("checkEmailDesc", { email })}
          </p>
          <p className="text-xs text-gray-500">
            {t("didntReceive")}{" "}
            <button
              type="button"
              className="text-brand-muted hover:text-brand-subtle underline"
              onClick={async () => {
                await fetch(`${API_URL}/auth/resend-verification`, {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ email }),
                })
              }}
            >
              {t("resendVerification")}
            </button>
          </p>
          <Link href="/login" className="block text-sm text-brand-muted hover:text-brand-subtle">
            {t("backToSignIn")}
          </Link>
        </div>
        </main>
        <Footer />
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col bg-gray-950">
      <main className="flex flex-1 items-center justify-center px-4 py-8">
      <div className="w-full max-w-md space-y-8 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h2 className="text-center text-3xl font-bold text-foreground">{t("title")}</h2>
          <p className="mt-2 text-center text-sm text-gray-400">{t("subtitle")}</p>
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
                {t("email")}
              </label>
              <input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => handleChange("email", e.target.value)}
                onBlur={(e) => { touchField("email"); validateField("email", e.target.value) }}
                className={fieldClass(fieldErrors.email)}
              />
              {fieldErrors.email && (
                <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.email}</p>
              )}
            </div>

            <div>
              <label htmlFor="username" className="block text-sm font-medium text-gray-300">
                {t("username")}
              </label>
              <div className="relative">
                <input
                  id="username"
                  type="text"
                  required
                  value={username}
                  onChange={(e) => handleChange("username", e.target.value)}
                  onBlur={(e) => { touchField("username"); validateField("username", e.target.value) }}
                  className={fieldClass(fieldErrors.username)}
                  placeholder="lowercase_letters_digits"
                />
                {usernameChecking && (
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 text-xs">…</span>
                )}
                {!usernameChecking && usernameAvailable === true && (
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-green-400" aria-label="Available">
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                    </svg>
                  </span>
                )}
                {!usernameChecking && usernameAvailable === false && (
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-red-400" aria-label="Unavailable">
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </span>
                )}
              </div>
              {fieldErrors.username && (
                <p className="mt-1 text-xs text-red-400" role="alert">{fieldErrors.username}</p>
              )}
              {!fieldErrors.username && usernameAvailable === false && (
                <p className="mt-1 text-xs text-red-400" role="alert">{t("usernameTaken")}</p>
              )}
            </div>

            <DateOfBirthPicker
              id="date_of_birth"
              label={t("dateOfBirth")}
              value={dateOfBirth}
              onChange={(v) => { handleChange("date_of_birth", v) }}
              onBlur={() => { touchField("date_of_birth"); validateField("date_of_birth", dateOfBirth) }}
              error={fieldErrors.date_of_birth}
              labels={{ month: t("dobMonth"), day: t("dobDay"), year: t("dobYear") }}
            />

            <div>
              <PasswordField
                id="password"
                label={t("password")}
                required
                value={password}
                onChange={(e) => handleChange("password", e.target.value)}
                onBlur={(e) => { touchField("password"); validateField("password", e.target.value); setPasswordFocused(false) }}
                onFocus={() => setPasswordFocused(true)}
                error={fieldErrors.password}
                autoComplete="new-password"
              />
              {(passwordFocused || password) && (
                <div className="mt-2">
                  <PasswordRequirements password={password} />
                </div>
              )}
            </div>

            <PasswordField
              id="confirm"
              label={t("confirmPassword")}
              required
              value={confirm}
              onChange={(e) => handleChange("confirm", e.target.value)}
              onBlur={(e) => { touchField("confirm"); validateField("confirm", e.target.value) }}
              error={fieldErrors.confirm}
              autoComplete="new-password"
            />

            <Checkbox
              ref={consentRef}
              id="accept_terms"
              checked={acceptTerms}
              onChange={(e) => {
                setAcceptTerms(e.target.checked)
                if (e.target.checked) setConsentError(undefined)
              }}
              error={consentError}
              label={t.rich("consentLabel", {
                terms: (chunks) => (
                  <Link
                    href="/terms"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-brand-muted underline hover:text-brand-subtle"
                  >
                    {chunks}
                  </Link>
                ),
                privacy: (chunks) => (
                  <Link
                    href="/privacy"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-brand-muted underline hover:text-brand-subtle"
                  >
                    {chunks}
                  </Link>
                ),
              })}
            />
          </div>

          <button
            type="submit"
            disabled={loading || !allRulesMet || usernameAvailable === false}
            className="w-full rounded-md bg-brand-primary px-4 py-3 text-foreground hover:bg-brand-hover disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-brand-hover focus:ring-offset-2 focus:ring-offset-gray-900"
          >
            {loading ? t("submitting") : t("submit")}
          </button>
        </form>

        <p className="text-center text-sm text-gray-400">
          {t("alreadyHaveAccount")}{" "}
          <Link href="/login" className="font-medium text-brand-muted hover:text-brand-subtle">
            {t("signIn")}
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
