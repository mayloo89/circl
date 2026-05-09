"use client"

import { signOut, useSession } from "next-auth/react"
import { useEffect, useState } from "react"
import { useTranslations, useLocale } from "next-intl"
import { useRouter, usePathname } from "@/i18n/navigation"
import { routing, type Locale } from "@/i18n/routing"

import { usePushContext } from "@/contexts/PushContext"
import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import Modal from "@/components/ui/Modal"
import PasswordField from "@/components/ui/PasswordField"
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
// Blocked users section
// ---------------------------------------------------------------------------

interface BlockedUser {
  block_id: string
  user_id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

function BlockedUsersSection({ token }: { token: string | undefined }) {
  const t = useTranslations("settings")
  const tc = useTranslations("common")
  const router = useRouter()
  const [blocked, setBlocked] = useState<BlockedUser[] | null>(null)
  const [unblockConfirm, setUnblockConfirm] = useState<BlockedUser | null>(null)
  const [pendingId, setPendingId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!token) return
    fetch(`${API_URL}/contacts/blocked`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => (r.ok ? r.json() : []))
      .then((data: BlockedUser[]) => setBlocked(data))
      .catch(() => setBlocked([]))
  }, [token])

  async function handleUnblock(userId: string) {
    if (!token) return
    setPendingId(userId)
    setError(null)
    try {
      const res = await fetch(`${API_URL}/contacts/${userId}/block`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        setError(t("unblockFailed"))
        return
      }
      setBlocked((prev) => prev?.filter((b) => b.user_id !== userId) ?? null)
      setUnblockConfirm(null)
    } catch {
      setError(tc("networkError"))
    } finally {
      setPendingId(null)
    }
  }

  return (
    <section aria-labelledby="blocked-heading">
      <h2 id="blocked-heading" className="mb-4 text-base font-semibold text-white">
        {t("blockedUsers")}
      </h2>
      <div className="rounded-lg bg-gray-800 ring-1 ring-gray-700">
        <div className="px-5 py-4">
          <p className="text-sm font-medium text-gray-200">{t("blockedUsersLabel")}</p>
          <p className="mt-0.5 text-xs text-gray-500">{t("blockedUsersDesc")}</p>
        </div>
        {error && (
          <p role="alert" className="border-t border-gray-700 px-5 py-3 text-sm text-red-400">
            {error}
          </p>
        )}
        {blocked === null ? (
          <p className="border-t border-gray-700 px-5 py-4 text-sm text-gray-500">{tc("loading")}</p>
        ) : blocked.length === 0 ? (
          <p className="border-t border-gray-700 px-5 py-4 text-sm text-gray-500">{t("blockedEmpty")}</p>
        ) : (
          <ul className="divide-y divide-gray-700 border-t border-gray-700">
            {blocked.map((b) => (
              <li key={b.block_id} className="flex items-center justify-between gap-3 px-5 py-3">
                <button
                  type="button"
                  onClick={() => router.push(`/profile/${b.username || b.user_id}`)}
                  className="flex items-center gap-3 rounded transition-opacity hover:opacity-80 focus:outline-none focus:ring-2 focus:ring-brand-hover"
                >
                  <Avatar src={b.avatar_url} name={b.display_name || b.email} size="sm" />
                  <span className="text-sm font-medium text-gray-300">{b.display_name || b.email}</span>
                </button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setUnblockConfirm(b)}
                  loading={pendingId === b.user_id}
                  disabled={pendingId !== null}
                >
                  {t("unblock")}
                </Button>
              </li>
            ))}
          </ul>
        )}
      </div>
      <ConfirmDialog
        open={unblockConfirm !== null}
        title={t("unblock")}
        message={t("unblockConfirmMessage", { name: unblockConfirm?.display_name || "" })}
        confirmLabel={t("unblock")}
        onConfirm={() => unblockConfirm && handleUnblock(unblockConfirm.user_id)}
        onCancel={() => setUnblockConfirm(null)}
      />
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
              <PasswordField
                id="current-password"
                label={t("currentPassword")}
                autoComplete="current-password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                required
                autoFocus
              />
              <div className="flex flex-col gap-2">
                <PasswordField
                  id="new-password"
                  label={t("newPassword")}
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                />
                <PasswordRequirements password={newPassword} />
              </div>
              <PasswordField
                id="confirm-password"
                label={t("confirmNewPassword")}
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
  const currentLocale = useLocale()
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
                aria-pressed={locale === currentLocale}
                className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-brand-hover ${
                  locale === currentLocale
                    ? "bg-brand-primary text-white"
                    : "text-gray-300 hover:bg-gray-700 hover:text-white"
                }`}
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
              className="cursor-pointer text-gray-500 transition-colors hover:text-gray-300 focus:outline-none focus:ring-2 focus:ring-brand-hover rounded"
            >
              <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div className="space-y-4 p-5">
            <p className="text-sm text-gray-400">{t("deleteModalDesc")}</p>
            <PasswordField
              id="delete-password"
              label={t("deletePassword")}
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
          <BlockedUsersSection token={token} />
          <PasswordSection token={token} />
          <DeleteAccountSection token={token} />
        </div>
      </div>
    </div>
  )
}
