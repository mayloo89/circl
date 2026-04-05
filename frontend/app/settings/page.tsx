"use client"

import { signOut, useSession } from "next-auth/react"
import { useState } from "react"

import { usePushContext } from "@/contexts/PushContext"
import Button from "@/components/ui/Button"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import Input from "@/components/ui/Input"
import PasswordRequirements, { PASSWORD_RULES } from "@/components/ui/PasswordRequirements"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

// ---------------------------------------------------------------------------
// Notifications section
// ---------------------------------------------------------------------------

function NotificationsSection() {
  const { permission, supported, enable, disable } = usePushContext()

  if (!supported) return null

  return (
    <section aria-labelledby="notifications-heading">
      <h2
        id="notifications-heading"
        className="mb-4 text-base font-semibold text-white"
      >
        Notifications
      </h2>
      <div className="rounded-lg bg-gray-800 ring-1 ring-gray-700">
        <div className="flex items-center justify-between px-5 py-4">
          <div>
            <p className="text-sm font-medium text-gray-200">
              Push notifications
            </p>
            <p className="mt-0.5 text-xs text-gray-500">
              Receive alerts for new messages and contact requests even when
              the app is closed.
            </p>
          </div>
          {permission === "granted" ? (
            <Button variant="secondary" size="sm" onClick={disable}>
              Disable
            </Button>
          ) : (
            <Button variant="primary" size="sm" onClick={enable}>
              Enable
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
  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const allRulesMet = PASSWORD_RULES.every(({ test }) => test(newPassword))

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSuccess(false)

    if (!allRulesMet) {
      setError("New password does not meet the requirements.")
      return
    }
    if (newPassword !== confirmPassword) {
      setError("New passwords do not match.")
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
        const data = await res.json().catch(() => ({}))
        setError(
          (data as { error?: string }).error ??
            "Failed to update password. Please try again.",
        )
        return
      }

      setSuccess(true)
      setCurrentPassword("")
      setNewPassword("")
      setConfirmPassword("")
    } catch {
      setError("Network error. Please try again.")
    } finally {
      setLoading(false)
    }
  }

  return (
    <section aria-labelledby="password-heading">
      <h2
        id="password-heading"
        className="mb-4 text-base font-semibold text-white"
      >
        Change password
      </h2>
      <div className="rounded-lg bg-gray-800 px-5 py-5 ring-1 ring-gray-700">
        <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
          <Input
            id="current-password"
            label="Current password"
            type="password"
            autoComplete="current-password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            required
          />
          <div className="flex flex-col gap-2">
            <Input
              id="new-password"
              label="New password"
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
            label="Confirm new password"
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
              Password updated successfully.
            </p>
          )}

          <div className="flex justify-end">
            <Button type="submit" variant="primary" size="md" loading={loading}>
              Update password
            </Button>
          </div>
        </form>
      </div>
    </section>
  )
}

// ---------------------------------------------------------------------------
// Delete account section
// ---------------------------------------------------------------------------

function DeleteAccountSection({ token }: { token: string | undefined }) {
  const [expanded, setExpanded] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function startDelete() {
    setPassword("")
    setError(null)
    setExpanded(true)
  }

  function cancel() {
    setExpanded(false)
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
        const data = await res.json().catch(() => ({}))
        setError(
          (data as { error?: string }).error ??
            "Failed to delete account. Please try again.",
        )
        setLoading(false)
        return
      }

      await signOut({ callbackUrl: "/login" })
    } catch {
      setError("Network error. Please try again.")
      setLoading(false)
    }
  }

  return (
    <section aria-labelledby="danger-heading">
      <h2
        id="danger-heading"
        className="mb-4 text-base font-semibold text-red-400"
      >
        Danger zone
      </h2>
      <div className="rounded-lg bg-gray-800 ring-1 ring-red-900">
        <div className="flex items-center justify-between px-5 py-4">
          <div>
            <p className="text-sm font-medium text-gray-200">Delete account</p>
            <p className="mt-0.5 text-xs text-gray-500">
              Permanently deactivates your account. This cannot be undone.
            </p>
          </div>
          {!expanded && (
            <Button variant="danger" size="sm" onClick={startDelete}>
              Delete account
            </Button>
          )}
        </div>

        {expanded && (
          <div className="border-t border-gray-700 px-5 pb-5 pt-4">
            <p className="mb-4 text-sm text-gray-400">
              Enter your password to confirm. You will be signed out
              immediately and will not be able to log back in.
            </p>
            <Input
              id="delete-password"
              label="Password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={error ?? undefined}
            />
            <div className="mt-4 flex justify-end gap-3">
              <Button variant="ghost" size="sm" onClick={cancel} disabled={loading}>
                Cancel
              </Button>
              <Button
                variant="danger"
                size="sm"
                loading={loading}
                onClick={() => {
                  if (!password) {
                    setError("Password is required.")
                    return
                  }
                  setError(null)
                  setConfirmOpen(true)
                }}
              >
                Delete my account
              </Button>
            </div>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={confirmOpen}
        title="Are you absolutely sure?"
        message="This action is irreversible. Your account will be permanently deactivated."
        confirmLabel="Yes, delete my account"
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
  const { data: session } = useSession()
  const token = session?.accessToken

  return (
    <div className="min-h-screen bg-gray-950 px-4 py-10">
      <div className="mx-auto max-w-2xl">
        <h1 className="mb-8 text-2xl font-bold text-white">Settings</h1>

        <div className="flex flex-col gap-10">
          <NotificationsSection />
          <PasswordSection token={token} />
          <DeleteAccountSection token={token} />
        </div>
      </div>
    </div>
  )
}
