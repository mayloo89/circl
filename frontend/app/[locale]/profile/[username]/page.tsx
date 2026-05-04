"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useParams } from "next/navigation"
import { useRouter } from "@/i18n/navigation"
import { useEffect, useRef, useState } from "react"
import { useTranslations } from "next-intl"

import Button from "@/components/ui/Button"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import ReportDialog from "@/components/ui/ReportDialog"
import Skeleton from "@/components/ui/Skeleton"
import PhotoGallery from "@/components/profile/PhotoGallery"
import Lightbox from "@/components/chat/Lightbox"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface ProfilePhoto {
  id: string
  url: string
}

interface PublicProfile {
  user_id: string
  username: string
  display_name: string
  bio: string
  avatar_url: string
  date_of_birth?: string
  gender: string
  location_text: string
  interests: string[]
  photos: ProfilePhoto[]
}

interface SentRequest     { contact_id: string; user_id: string }
interface AcceptedContact { contact_id: string; user_id: string }
interface PendingRequest  { contact_id: string; user_id: string }
interface BlockedUser     { block_id: string;   user_id: string }

type ContactStatus = "none" | "contact" | "sent" | "incoming" | "loading"

function formatAge(dob?: string): string | null {
  if (!dob) return null
  const d = new Date(dob)
  const today = new Date()
  let age = today.getFullYear() - d.getFullYear()
  const m = today.getMonth() - d.getMonth()
  if (m < 0 || (m === 0 && today.getDate() < d.getDate())) age--
  return String(age)
}

function HeroSkeleton() {
  return (
    <div>
      <div className="min-h-[55vh] animate-pulse bg-gray-800" />
      <div className="space-y-4 px-4 pt-6">
        <div className="space-y-2">
          <Skeleton className="h-7 w-40" />
          <Skeleton className="h-4 w-28" />
        </div>
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
        <div className="flex gap-2">
          <Skeleton className="h-6 w-20 rounded-full" />
          <Skeleton className="h-6 w-16 rounded-full" />
          <Skeleton className="h-6 w-24 rounded-full" />
        </div>
      </div>
    </div>
  )
}

export default function PublicProfilePage() {
  const { data: session, status } = useSession()
  const router = useRouter()
  const params = useParams()
  const username = typeof params.username === "string" ? params.username : null

  const [profile, setProfile] = useState<PublicProfile | null>(null)
  const [contactStatus, setContactStatus] = useState<ContactStatus>("loading")
  const [contactId, setContactId] = useState<string | null>(null)
  const [isBlocked, setIsBlocked] = useState(false)
  const [blockConfirmOpen, setBlockConfirmOpen] = useState(false)
  const [reportConfirmOpen, setReportConfirmOpen] = useState(false)
  const [reportError, setReportError] = useState("")
  const [reportSuccess, setReportSuccess] = useState(false)
  const [lightbox, setLightbox] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)
  const [overflowOpen, setOverflowOpen] = useState(false)
  const overflowRef = useRef<HTMLDivElement>(null)

  const t = useTranslations("publicProfile")
  const tc = useTranslations("common")
  const token = session?.accessToken
  const myID = session?.user?.id

  useEffect(() => {
    if (!overflowOpen) return
    function handleClick(e: MouseEvent) {
      if (overflowRef.current && !overflowRef.current.contains(e.target as Node)) {
        setOverflowOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [overflowOpen])

  useEffect(() => {
    if (status !== "authenticated" || !token || !username) return

    Promise.all([
      fetch(`${API_URL}/profiles/${username}`, { headers: { Authorization: `Bearer ${token}` } }),
      fetch(`${API_URL}/contacts`,          { headers: { Authorization: `Bearer ${token}` } }),
      fetch(`${API_URL}/contacts/sent`,     { headers: { Authorization: `Bearer ${token}` } }),
      fetch(`${API_URL}/contacts/pending`,  { headers: { Authorization: `Bearer ${token}` } }),
      fetch(`${API_URL}/contacts/blocked`,  { headers: { Authorization: `Bearer ${token}` } }),
    ])
      .then(async ([profileRes, contactsRes, sentRes, pendingRes, blockedRes]) => {
        if (profileRes.status === 404) { setError(t("notFound")); return }
        if (!profileRes.ok) throw new Error(t("failedLoad"))

        const [prof, contacts, sent, pending, blocked]: [
          PublicProfile,
          AcceptedContact[],
          SentRequest[],
          PendingRequest[],
          BlockedUser[],
        ] = await Promise.all([
          profileRes.json(),
          contactsRes.ok  ? contactsRes.json()  : [],
          sentRes.ok      ? sentRes.json()      : [],
          pendingRes.ok   ? pendingRes.json()   : [],
          blockedRes.ok   ? blockedRes.json()   : [],
        ])

        if (myID && prof.user_id === myID) { router.replace("/profile"); return }

        setProfile(prof)
        const targetID = prof.user_id

        const blockEntry = blocked.find((b) => b.user_id === targetID)
        if (blockEntry) { setIsBlocked(true); return }

        const accepted = contacts.find((c) => c.user_id === targetID)
        if (accepted) { setContactStatus("contact"); setContactId(accepted.contact_id); return }
        const sentEntry = sent.find((s) => s.user_id === targetID)
        if (sentEntry) { setContactStatus("sent"); setContactId(sentEntry.contact_id); return }
        const incomingEntry = pending.find((p) => p.user_id === targetID)
        if (incomingEntry) { setContactStatus("incoming"); setContactId(incomingEntry.contact_id); return }
        setContactStatus("none")
      })
      .catch(() => setError(t("failedLoad")))
      .finally(() => setLoading(false))
  }, [status, token, username, myID, router, t])

  async function handleAddContact() {
    if (!profile) return
    setActionLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/contacts`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ addressee_id: profile.user_id }),
      })
      if (res.status === 409) { setContactStatus("sent"); return }
      if (!res.ok) { setError(t("failedContact")); return }
      const contact = await res.json()
      setContactId(contact.id)
      setContactStatus("sent")
    } finally {
      setActionLoading(false)
    }
  }

  async function handleAccept() {
    if (!contactId) return
    setActionLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/contacts/${contactId}/accept`, {
        method: "PUT",
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) { setError(t("failedAccept")); return }
      setContactStatus("contact")
    } finally {
      setActionLoading(false)
    }
  }

  async function handleStartDM() {
    if (!profile) return
    setActionLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/chat/rooms/dm`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ peer_id: profile.user_id }),
      })
      if (!res.ok) { setError(t("failedMessage")); return }
      const room = await res.json()
      router.push(`/chat/${room.id}`)
    } finally {
      setActionLoading(false)
    }
  }

  async function handleBlock() {
    if (!profile) return
    setActionLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/contacts/${profile.user_id}/block`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) { setError(t("failedBlock")); return }
      setIsBlocked(true)
      setBlockConfirmOpen(false)
    } finally {
      setActionLoading(false)
    }
  }

  async function handleUnblock() {
    if (!profile) return
    setActionLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/contacts/${profile.user_id}/block`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) { setError(t("failedUnblock")); return }
      setIsBlocked(false)
      setContactStatus("none")
    } finally {
      setActionLoading(false)
    }
  }

  async function handleReport(reason: string, description: string) {
    if (!profile) return
    setActionLoading(true)
    setReportError("")
    try {
      const res = await fetch(`${API_URL}/reports`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ reported_user_id: profile.user_id, reason, description }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        setReportError(data.error || t("failedReport"))
        return
      }
      setReportConfirmOpen(false)
      setReportSuccess(true)
    } finally {
      setActionLoading(false)
    }
  }

  if (status === "loading" || loading) {
    return <HeroSkeleton />
  }

  if (error || !profile) {
    return (
      <div className="px-4 py-8">
        <button
          type="button"
          onClick={() => router.back()}
          className="mb-6 flex items-center gap-1.5 text-sm text-gray-400 hover:text-white"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
          </svg>
          {tc("back")}
        </button>
        <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">
          {error || t("notFound")}
        </p>
      </div>
    )
  }

  const age = formatAge(profile.date_of_birth)
  const subtitle = [age, profile.gender].filter(Boolean).join(" · ")

  const ActionButtons = ({ className = "" }: { className?: string }) => {
    if (isBlocked) {
      return (
        <Button variant="ghost" className={className} onClick={handleUnblock} disabled={actionLoading} loading={actionLoading}>
          {t("actions.unblock")}
        </Button>
      )
    }
    return (
      <div className={`flex gap-3 ${className}`}>
        {contactStatus === "contact" && (
          <Button variant="accent" className="flex-1 lg:flex-none" onClick={handleStartDM} disabled={actionLoading} loading={actionLoading}>
            {t("actions.message")}
          </Button>
        )}
        {contactStatus === "none" && (
          <Button variant="accent" className="flex-1 lg:flex-none" onClick={handleAddContact} disabled={actionLoading} loading={actionLoading}>
            {t("actions.addContact")}
          </Button>
        )}
        {contactStatus === "sent" && (
          <span className="flex flex-1 items-center justify-center rounded-full bg-gray-800 px-5 py-2 text-sm text-gray-400 ring-1 ring-gray-700 lg:flex-none">
            {t("requestSent")}
          </span>
        )}
        {contactStatus === "incoming" && (
          <Button variant="success" className="flex-1 lg:flex-none" onClick={handleAccept} disabled={actionLoading} loading={actionLoading}>
            {t("acceptRequest")}
          </Button>
        )}
      </div>
    )
  }

  return (
    <>
      {lightbox && (
        <Lightbox url={lightbox} type="image" onClose={() => setLightbox(null)} />
      )}

      <ConfirmDialog
        open={blockConfirmOpen}
        title={t("blockUserTitle")}
        message={t("blockUserMessage", { name: profile.display_name })}
        confirmLabel={t("block")}
        loading={actionLoading}
        onConfirm={handleBlock}
        onCancel={() => setBlockConfirmOpen(false)}
      />

      <ReportDialog
        open={reportConfirmOpen}
        loading={actionLoading}
        error={reportError}
        onSubmit={handleReport}
        onCancel={() => setReportConfirmOpen(false)}
      />

      {/* ── Hero ── */}
      <div className="relative min-h-[55vh] overflow-hidden bg-gray-900">
        {profile.avatar_url ? (
          <button
            type="button"
            className="absolute inset-0 cursor-zoom-in"
            aria-label={t("viewPhoto")}
            onClick={() => setLightbox(profile.avatar_url)}
          >
            <Image
              src={profile.avatar_url}
              alt={profile.display_name}
              fill
              sizes="100vw"
              className="object-cover object-center"
              priority
            />
          </button>
        ) : (
          <div className="absolute inset-0 flex items-center justify-center bg-gradient-to-br from-indigo-950 via-brand-primary/40 to-gray-900">
            <span className="select-none text-[8rem] font-black text-white/20">
              {profile.display_name?.[0]?.toUpperCase() ?? "?"}
            </span>
          </div>
        )}

        {/* bottom-fade overlay so content below blends in */}
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-black/30 via-transparent to-gray-950" />

        {/* Back */}
        <button
          type="button"
          onClick={() => router.back()}
          aria-label={tc("back")}
          className="absolute left-4 top-4 z-10 flex h-10 w-10 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-sm transition-colors hover:bg-black/70 focus:outline-none focus:ring-2 focus:ring-white/50"
        >
          <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
          </svg>
        </button>

        {/* Overflow menu */}
        <div ref={overflowRef} className="absolute right-4 top-4 z-10">
          <button
            type="button"
            onClick={() => setOverflowOpen((v) => !v)}
            aria-label={t("moreOptions")}
            aria-expanded={overflowOpen}
            className="flex h-10 w-10 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-sm transition-colors hover:bg-black/70 focus:outline-none focus:ring-2 focus:ring-white/50"
          >
            <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <circle cx="12" cy="5"  r="1.5" />
              <circle cx="12" cy="12" r="1.5" />
              <circle cx="12" cy="19" r="1.5" />
            </svg>
          </button>

          {overflowOpen && (
            <div
              role="menu"
              className="absolute right-0 mt-1 min-w-[10rem] overflow-hidden rounded-xl bg-gray-800 shadow-2xl ring-1 ring-gray-700"
            >
              {isBlocked ? (
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => { setOverflowOpen(false); void handleUnblock() }}
                  disabled={actionLoading}
                  className="w-full px-4 py-2.5 text-left text-sm text-gray-200 hover:bg-gray-700 disabled:opacity-50"
                >
                  {t("actions.unblock")}
                </button>
              ) : (
                <>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => { setOverflowOpen(false); setBlockConfirmOpen(true) }}
                    className="w-full px-4 py-2.5 text-left text-sm text-red-400 hover:bg-gray-700"
                  >
                    {t("actions.block")}
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => { setOverflowOpen(false); setReportConfirmOpen(true) }}
                    className="w-full border-t border-gray-700 px-4 py-2.5 text-left text-sm text-gray-300 hover:bg-gray-700"
                  >
                    {t("actions.report")}
                  </button>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* ── Content ── */}
      <div className="space-y-5 px-4 pb-36 pt-6 lg:pb-8">

        {error && (
          <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">{error}</p>
        )}
        {reportSuccess && (
          <p className="rounded-md bg-green-950 p-3 text-sm text-green-400 ring-1 ring-green-900">{t("reportSuccess")}</p>
        )}

        {/* Name + subtitle */}
        <div>
          <h1 className="text-2xl font-bold text-white">{profile.display_name}</h1>
          {subtitle && <p className="mt-0.5 text-sm text-gray-400">{subtitle}</p>}
        </div>

        {/* Location */}
        {profile.location_text && (
          <p className="flex items-center gap-1.5 text-sm text-gray-400">
            <svg className="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            {profile.location_text}
          </p>
        )}

        {/* Bio */}
        {profile.bio && (
          <p className="text-sm leading-relaxed text-gray-300">{profile.bio}</p>
        )}

        {/* Interests */}
        {profile.interests?.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {profile.interests.map((tag) => (
              <span
                key={tag}
                className="rounded-full bg-brand-wash/50 px-3 py-1 text-xs text-brand-subtle ring-1 ring-brand-strong/60"
              >
                {tag}
              </span>
            ))}
          </div>
        )}

        {/* Desktop inline actions */}
        {contactStatus !== "loading" && (
          <div className="hidden lg:block pt-1">
            <ActionButtons />
          </div>
        )}

        <PhotoGallery photos={profile.photos} onPhotoClick={setLightbox} />
      </div>

      {/* ── Mobile sticky action bar ── */}
      {contactStatus !== "loading" && (
        <div
          className="fixed inset-x-0 z-30 border-t border-gray-800 bg-gray-950/95 px-4 py-3 backdrop-blur-sm lg:hidden"
          style={{
            bottom: "calc(4rem + env(safe-area-inset-bottom, 0px))",
            paddingBottom: "0.75rem",
          }}
        >
          <ActionButtons />
        </div>
      )}
    </>
  )
}
