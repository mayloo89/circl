"use client"

import { signOut, useSession } from "next-auth/react"
import { useState } from "react"
import { useTranslations } from "next-intl"
import { useRouter, usePathname } from "@/i18n/navigation"
import { routing, type Locale } from "@/i18n/routing"

import { usePushContext } from "@/contexts/PushContext"
import Button from "@/components/ui/Button"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import Input from "@/components/ui/Input"
import Modal from "@/components/ui/Modal"
import PasswordRequirements, { PASSWORD_RULES } from "@/components/ui/PasswordRequirements"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

// ---------------------------------------------------------------------------
// Notifications section
// ---------------------------------------------------------------------------

function NotificationsSection() {
  const t = useTranslations("settings")
  const { permission, supported, enable, disable } = usePushContext()

  if (!supported) return null

  return (
    <section aria-labelledby="notifications-heading">
      <h2 id="notifications-heading" className="mb-4 text-base font-semibold text-white">
        {t("notifications")}
      </h2>
      <div className="rounded-lg bg-gray-800 ring-1 ring-gray-700">
        <div className="flex items-center justify-between px-5 py-4">
          <div>
            <p className="text-sm font-medium text-gray-200">{t("pushNotifications")}</p>
            <p className="mt-0.5 text-xs text-gray-500">{t("pushNotificationsDesc")}</p>
          </div>
          {permission === "granted" ? (
            <Button variant="secondary" size="sm" onClick={disable}>
              {t("disable")}
            </Button>
          ) : (
            <Button variant="primary" size="sm" onClick={enable}>
              {t("enable")}
            </Button>
          )}
        </div>
      </div>
    </section>
  )
}

// ---------------------------------------------------------------------------
// Change password section
// ---------------------------------------------------------------------------

function PasswordSection({ token }: { token: string | undefined }) {
  const t = useTranslations("settings")
  const tc = useTranslations("common")
  const [expanded, setExpanded] = useState(false)
  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const allRulesMet = PASSWORD_RULES.every(({ test }) => test(newPassword))

  function cancel() {
    setExpanded(false)
    setCurrentPassword("")
    setNewPassword("")
    setConfirmPassword("")
    setError(null)
    setSuccess(false)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSuccess(false)

    if (!allRulesMet) {
      setError(t("passwordRequirements"))
      return
    }
    if (newPassword !== confirmPassword) {
      setError(t("passwordsMismatch"))
      return
    }

    setLoading(true)
    try {
      const res = await fetch(`${API_URL}/users/me/password`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      })

      if (!res.ok) {
        setError(t("passwordFailed"))
        return
      }

      setSuccess(true)
      setCurrentPassword("")
      setNewPassword("")
      setConfirmPassword("")
    } catch {
      setError(tc("networkError"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <section aria-labelledby="password-heading">
      <h2 id="password-heading" className="mb-4 text-base font-semibold text-white">
        {t("changePassword")}
      </h2>
      <div className="rounded-lg bg-gray-800 ring-1 ring-gray-700">
        <div className="flex items-center justify-between px-5 py-4">
          <div>
            <p className="text-sm font-medium text-gray-200">{t("passwordLabel")}</p>
            <p className="mt-0.5 text-xs text-gray-500">{t("passwordDesc")}</p>
          </div>
          {!expanded && (
            <Button variant="secondary" size="sm" onClick={() => setExpanded(true)}>
              {t("changePassword")}
            </Button>
          )}
        </div>

        {expanded && (
          <div className="border-t border-gray-700 px-5 pb-5 pt-4">
            <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
              <Input
                id="current-password"
                label={t("currentPassword")}
                type="password"
                autoComplete="current-password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                required
                autoFocus
              />
              <div className="flex flex-col gap-2">
                <Input
                  id="new-password"
                  label={t("newPassword")}
                  type="password"
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                />
                <PasswordRequirements password={newPassword} />
              </div>
              <Input
                id="confirm-password"
                label={t("confirmNewPassword")}
                type="password"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
              />

              {error && (
                <p role="alert" className="text-sm text-red-400">
                  {error}
                </p>
              )}
              {success && (
                <p role="status" className="text-sm text-green-400">
                  {t("passwordUpdated")}
                </p>
              )}

              <div className="flex justify-end gap-3">
                <Button variant="ghost" size="md" onClick={cancel} disabled={loading} type="button">
                  {tc("cancel")}
                </Button>
                <Button type="submit" variant="primary" size="md" loading={loading}>
                  {t("updatePassword")}
                </Button>
              </div>
            </form>
          </div>
        )}
      </div>
    </section>
  )
}

// ---------------------------------------------------------------------------
// Language section
// ---------------------------------------------------------------------------

const LOCALE_LABEL_KEYS: Record<Locale, "languageEs" | "languageEn" | "languagePt"> = {
  es: "languageEs",
  en: "languageEn",
  pt: "languagePt",
}

function LanguageSection({ token }: { token: string | undefined }) {
  const t = useTranslations("settings")
  const router = useRouter()
  const pathname = usePathname()
  const [saving, setSaving] = useState(false)

  async function handleChange(locale: Locale) {
    setSaving(true)
    if (token) {
      try {
        await fetch(`${API_URL}/profiles/me/preferences`, {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ locale }),
        })
      } catch {
        // non-critical — still switch the UI locale
      }
    }
    setSaving(false)
    router.replace(pathname, { locale })
  }

  return (
    <section aria-labelledby="language-heading">
      <h2 id="language-heading" className="mb-4 text-base font-semibold text-white">
        {t("language")}
      </h2>
      <div className="rounded-lg bg-gray-800 ring-1 ring-gray-700">
        <div className="flex items-center justify-between px-5 py-4">
          <div>
            <p className="text-sm font-medium text-gray-200">{t("language")}</p>
            <p className="mt-0.5 text-xs text-gray-500">{t("languageDesc")}</p>
          </div>
          <div className="flex gap-2">
            {routing.locales.map((locale) => (
              <button
                key={locale}
                onClick={() => handleChange(locale)}
                disabled={saving}
                className="rounded-md px-3 py-1.5 text-sm font-medium text-gray-300 hover:bg-gray-700 hover:text-white disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-indigo-500"
              >
                {t(LOCALE_LABEL_KEYS[locale])}
              </button>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

// ---------------------------------------------------------------------------
// Delete account section
// ---------------------------------------------------------------------------

function DeleteAccountSection({ token }: { token: string | undefined }) {
  const t = useTranslations("settings")
  const tc = useTranslations("common")
  const [modalOpen, setModalOpen] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function openModal() {
    setPassword("")
    setError(null)
    setModalOpen(true)
  }

  function closeModal() {
    setModalOpen(false)
    setPassword("")
    setError(null)
  }

  async function handleDelete() {
    setConfirmOpen(false)
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`${API_URL}/users/me`, {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ password }),
      })

      if (!res.ok) {
        setError(t("deleteFailed"))
        setLoading(false)
        return
      }

      await signOut({ callbackUrl: "/login" })
    } catch {
      setError(tc("networkError"))
      setLoading(false)
    }
  }

  return (
    <section aria-labelledby="danger-heading">
      <h2 id="danger-heading" className="mb-4 text-base font-semibold text-red-400">
        {t("dangerZone")}
      </h2>
      <div className="rounded-lg bg-gray-800 ring-1 ring-red-900">
        <div className="flex items-center justify-between px-5 py-4">
          <div>
            <p className="text-sm font-medium text-gray-200">{t("deleteAccount")}</p>
            <p className="mt-0.5 text-xs text-gray-500">{t("deleteAccountDesc")}</p>
          </div>
          <Button variant="danger" size="sm" onClick={openModal}>
            {t("deleteButton")}
          </Button>
        </div>
      </div>

      <Modal open={modalOpen} onClose={closeModal}>
        <div
          className="w-full max-w-md rounded-xl bg-gray-900 shadow-2xl ring-1 ring-red-900 mx-4"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-center justify-between border-b border-gray-800 px-5 py-4">
            <h2 className="text-base font-semibold text-white">{t("deleteModalTitle")}</h2>
            <button
              type="button"
              aria-label={tc("close")}
              onClick={closeModal}
              className="text-gray-500 hover:text-gray-300"
            >
              ✕
            </button>
          </div>
          <div className="space-y-4 p-5">
            <p className="text-sm text-gray-400">{t("deleteModalDesc")}</p>
            <Input
              id="delete-password"
              label={t("deletePassword")}
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={error ?? undefined}
              autoFocus
            />
          </div>
          <div className="flex justify-end gap-3 border-t border-gray-800 px-5 py-4">
            <Button variant="ghost" size="sm" onClick={closeModal} disabled={loading}>
              {tc("cancel")}
            </Button>
            <Button
              variant="danger"
              size="sm"
              loading={loading}
              onClick={() => {
                if (!password) {
                  setError(tc("passwordRequired"))
                  return
                }
                setError(null)
                setConfirmOpen(true)
              }}
            >
              {t("deleteMyAccount")}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={confirmOpen}
        title={t("confirmDeleteTitle")}
        message={t("confirmDeleteMessage")}
        confirmLabel={t("confirmDeleteLabel")}
        onConfirm={handleDelete}
        onCancel={() => setConfirmOpen(false)}
      />
    </section>
  )
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function SettingsPage() {
  const t = useTranslations("settings")
  const { data: session } = useSession()
  const token = session?.accessToken

  return (
    <div className="min-h-screen bg-gray-950 px-4 py-10">
      <div className="mx-auto max-w-2xl">
        <h1 className="mb-8 text-2xl font-bold text-white">{t("title")}</h1>

        <div className="flex flex-col gap-10">
          <NotificationsSection />
          <LanguageSection token={token} />
          <PasswordSection token={token} />
          <DeleteAccountSection token={token} />
        </div>
      </div>
    </div>
  )
}
