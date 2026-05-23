"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"

import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import { albumsApi, type Grant, type GrantExpiryPreset } from "@/lib/albums"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface AcceptedContact {
  user_id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

interface Props {
  albumID: string
  token: string
}

export default function MembersPanel({ albumID, token }: Props) {
  const t = useTranslations("albums")
  const [grants, setGrants] = useState<Grant[]>([])
  const [contacts, setContacts] = useState<AcceptedContact[]>([])
  const [selected, setSelected] = useState("")
  const [expiresIn, setExpiresIn] = useState<GrantExpiryPreset>("none")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")
  const [confirmRevoke, setConfirmRevoke] = useState<Grant | null>(null)

  useEffect(() => {
    let cancelled = false
    Promise.all([
      albumsApi.listGrants(token, albumID).catch(() => [] as Grant[]),
      fetch(`${API_URL}/contacts`, { headers: { Authorization: `Bearer ${token}` } })
        .then((r) => (r.ok ? (r.json() as Promise<AcceptedContact[]>) : Promise.resolve([] as AcceptedContact[])))
        .catch(() => [] as AcceptedContact[]),
    ]).then(([g, c]) => {
      if (cancelled) return
      setGrants(g)
      setContacts(c)
    })
    return () => {
      cancelled = true
    }
  }, [albumID, token])

  // Contacts that don't already have an open or active grant. Mapped by id
  // so the picker shows display name + avatar.
  const openGranteeIDs = new Set(
    grants.filter((g) => g.status === "active" || (g.status === "pending" && g.source === "request")).map((g) => g.grantee_id),
  )
  const candidates = contacts.filter((c) => !openGranteeIDs.has(c.user_id))
  const contactByID = new Map(contacts.map((c) => [c.user_id, c]))

  async function invite() {
    if (!selected) return
    setSubmitting(true)
    setError("")
    try {
      const grant = await albumsApi.invite(token, albumID, selected, expiresIn)
      setGrants((prev) => [grant, ...prev])
      setSelected("")
      setExpiresIn("none")
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed")
    } finally {
      setSubmitting(false)
    }
  }

  async function revoke(grantID: string) {
    try {
      const updated = await albumsApi.revoke(token, grantID)
      setGrants((prev) => prev.map((g) => (g.id === grantID ? updated : g)))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed")
    }
  }

  async function accept(grantID: string) {
    try {
      const updated = await albumsApi.accept(token, grantID)
      setGrants((prev) => prev.map((g) => (g.id === grantID ? updated : g)))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed")
    }
  }

  async function deny(grantID: string) {
    try {
      const updated = await albumsApi.deny(token, grantID)
      setGrants((prev) => prev.map((g) => (g.id === grantID ? updated : g)))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed")
    }
  }

  function accessTimeLabel(expiresAt: string | null | undefined): string {
    if (!expiresAt) return t("accessPermanent")
    const diffMs = new Date(expiresAt).getTime() - Date.now()
    if (diffMs <= 0) return t("accessExpired")
    const days = Math.ceil(diffMs / (1000 * 60 * 60 * 24))
    if (days < 1) return t("accessToday")
    return t("accessDaysLeft", { count: days })
  }

  const expiryPresetKeys: Record<GrantExpiryPreset, Parameters<typeof t>[0]> = {
    "24h": "expiryPreset24h",
    "7d": "expiryPreset7d",
    "30d": "expiryPreset30d",
    none: "expiryPresetNone",
  }

  const open = grants.filter((g) => g.status === "active" || (g.status === "pending" && g.source === "request"))

  return (
    <section className="rounded-lg border border-gray-800 bg-gray-900/40 p-4 space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-gray-300">{t("members")}</h2>

      <div className="space-y-2">
        <div className="flex flex-wrap items-end gap-2">
          <label htmlFor="invite-select" className="sr-only">
            {t("invite")}
          </label>
          <select
            id="invite-select"
            value={selected}
            onChange={(e) => setSelected(e.target.value)}
            className="min-w-[12rem] flex-1 rounded border border-gray-700 bg-gray-950 px-3 py-2 text-sm text-white"
          >
            <option value="">{t("invitePlaceholder")}</option>
            {candidates.map((c) => (
              <option key={c.user_id} value={c.user_id}>
                {c.display_name || c.username}
              </option>
            ))}
          </select>
          <Button variant="primary" size="sm" onClick={invite} loading={submitting} disabled={!selected || submitting}>
            {t("inviteSubmit")}
          </Button>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-xs text-gray-500">{t("expiryLabel")}:</span>
          {(["24h", "7d", "30d", "none"] as GrantExpiryPreset[]).map((preset) => (
            <button
              key={preset}
              type="button"
              onClick={() => setExpiresIn(preset)}
              className={`rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors ${
                expiresIn === preset
                  ? "bg-brand-accent text-white"
                  : "bg-gray-800 text-gray-400 hover:bg-gray-700 hover:text-white"
              }`}
            >
              {t(expiryPresetKeys[preset])}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <p role="alert" className="text-sm text-red-400">
          {error}
        </p>
      )}

      {open.length === 0 ? (
        <p className="text-sm text-gray-500">{t("noMembers")}</p>
      ) : (
        <ul className="space-y-2">
          {open.map((g) => {
            const contact = contactByID.get(g.grantee_id)
            const isRequestPending = g.status === "pending" && g.source === "request"
            return (
              <li
                key={g.id}
                className="flex items-center gap-3 rounded border border-gray-800 bg-gray-950 p-3"
              >
                <Avatar
                  src={contact?.avatar_url ?? ""}
                  name={contact?.display_name ?? g.grantee_id}
                  size="sm"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm text-foreground">
                    {contact?.display_name || contact?.username || g.grantee_id}
                  </p>
                  <p className="text-xs text-gray-500">
                    {g.status === "active" ? t("memberStatusActive") : t("memberStatusRequested")}
                  </p>
                  {g.status === "active" && (
                    <p className={`text-xs ${g.expires_at ? "text-amber-400" : "text-gray-500"}`}>
                      {accessTimeLabel(g.expires_at)}
                    </p>
                  )}
                </div>
                <div className="flex shrink-0 gap-2">
                  {isRequestPending && (
                    <>
                      <Button variant="primary" size="sm" onClick={() => accept(g.id)}>
                        {t("acceptInvite")}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => deny(g.id)}>
                        {t("denyInvite")}
                      </Button>
                    </>
                  )}
                  {g.status === "active" && (
                    <Button variant="danger" size="sm" onClick={() => setConfirmRevoke(g)}>
                      {t("memberRevoke")}
                    </Button>
                  )}
                </div>
              </li>
            )
          })}
        </ul>
      )}

      <ConfirmDialog
        open={confirmRevoke !== null}
        title={t("memberRevokeConfirm")}
        message={t("memberRevokeBody")}
        confirmLabel={t("memberRevoke")}
        onConfirm={async () => {
          const g = confirmRevoke
          setConfirmRevoke(null)
          if (g) await revoke(g.id)
        }}
        onCancel={() => setConfirmRevoke(null)}
      />
    </section>
  )
}
