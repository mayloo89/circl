"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useParams, useRouter } from "next/navigation"
import { useEffect, useState } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface ProfilePhoto {
  id: string
  url: string
}

interface PublicProfile {
  user_id: string
  display_name: string
  bio: string
  avatar_url: string
  photos: ProfilePhoto[]
}

type ContactStatus = "none" | "contact" | "sent" | "incoming" | "loading"

interface SentRequest  { contact_id: string; user_id: string }
interface AcceptedContact { contact_id: string; user_id: string }
interface PendingRequest  { contact_id: string; user_id: string }

function ProfileSkeleton() {
  return (
    <div className="w-full max-w-lg space-y-6 px-4">
      <div className="flex items-center justify-between">
        <span className="h-8 w-24 animate-pulse rounded bg-gray-800" />
        <span className="h-5 w-16 animate-pulse rounded bg-gray-800" />
      </div>
      <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
        <div className="flex flex-col items-center gap-4">
          <span className="h-24 w-24 animate-pulse rounded-full bg-gray-800" />
          <span className="h-5 w-36 animate-pulse rounded bg-gray-800" />
          <span className="h-4 w-48 animate-pulse rounded bg-gray-800" />
          <span className="h-10 w-32 animate-pulse rounded-full bg-gray-800" />
        </div>
      </div>
      <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
        <div className="grid grid-cols-3 gap-3">
          {[0, 1, 2].map((i) => (
            <span key={i} className="aspect-square animate-pulse rounded-lg bg-gray-800" />
          ))}
        </div>
      </div>
    </div>
  )
}

export default function PublicProfilePage() {
  const { data: session, status } = useSession()
  const router = useRouter()
  const params = useParams()
  const userId = typeof params.userId === "string" ? params.userId : null

  const [profile, setProfile] = useState<PublicProfile | null>(null)
  const [contactStatus, setContactStatus] = useState<ContactStatus>("loading")
  const [contactId, setContactId] = useState<string | null>(null)
  const [lightbox, setLightbox] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)

  const token = session?.accessToken
  const myID = session?.user?.id

  useEffect(() => {
    if (status === "unauthenticated") router.push("/login")
  }, [status, router])

  // Redirect to own private profile if viewing self
  useEffect(() => {
    if (myID && userId && myID === userId) {
      router.replace("/profile")
    }
  }, [myID, userId, router])

  useEffect(() => {
    if (status !== "authenticated" || !token || !userId) return

    Promise.all([
      fetch(`${API_URL}/profiles/${userId}`, { headers: { Authorization: `Bearer ${token}` } }),
      fetch(`${API_URL}/contacts`, { headers: { Authorization: `Bearer ${token}` } }),
      fetch(`${API_URL}/contacts/sent`, { headers: { Authorization: `Bearer ${token}` } }),
      fetch(`${API_URL}/contacts/pending`, { headers: { Authorization: `Bearer ${token}` } }),
    ])
      .then(async ([profileRes, contactsRes, sentRes, pendingRes]) => {
        if (profileRes.status === 404) {
          setError("Profile not found.")
          return
        }
        if (!profileRes.ok) throw new Error("Failed to load profile.")

        const [prof, contacts, sent, pending]: [
          PublicProfile,
          AcceptedContact[],
          SentRequest[],
          PendingRequest[],
        ] = await Promise.all([
          profileRes.json(),
          contactsRes.ok ? contactsRes.json() : [],
          sentRes.ok ? sentRes.json() : [],
          pendingRes.ok ? pendingRes.json() : [],
        ])

        setProfile(prof)

        const accepted = contacts.find((c) => c.user_id === userId)
        if (accepted) {
          setContactStatus("contact")
          setContactId(accepted.contact_id)
          return
        }
        const sentEntry = sent.find((s) => s.user_id === userId)
        if (sentEntry) {
          setContactStatus("sent")
          setContactId(sentEntry.contact_id)
          return
        }
        const incomingEntry = pending.find((p) => p.user_id === userId)
        if (incomingEntry) {
          setContactStatus("incoming")
          setContactId(incomingEntry.contact_id)
          return
        }
        setContactStatus("none")
      })
      .catch(() => setError("Failed to load profile."))
      .finally(() => setLoading(false))
  }, [status, token, userId])

  async function handleAddContact() {
    if (!userId) return
    setActionLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/contacts`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ addressee_id: userId }),
      })
      if (res.status === 409) {
        setContactStatus("sent")
        return
      }
      if (!res.ok) {
        setError("Failed to send contact request.")
        return
      }
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
      if (!res.ok) {
        setError("Failed to accept contact.")
        return
      }
      setContactStatus("contact")
    } finally {
      setActionLoading(false)
    }
  }

  async function handleStartDM() {
    if (!userId) return
    setActionLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/chat/rooms/dm`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ peer_id: userId }),
      })
      if (!res.ok) {
        setError("Failed to open conversation.")
        return
      }
      const room = await res.json()
      router.push(`/chat/${room.id}`)
    } finally {
      setActionLoading(false)
    }
  }

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
        <ProfileSkeleton />
      </div>
    )
  }

  if (error || !profile) {
    return (
      <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
        <div className="w-full max-w-lg px-4">
          <div className="flex items-center justify-between">
            <h1 className="text-3xl font-bold text-white">Profile</h1>
            <button
              aria-label="Go back"
              onClick={() => router.back()}
              className="text-sm text-gray-400 hover:text-gray-200"
            >
              ← Back
            </button>
          </div>
          <p className="mt-6 rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">
            {error || "Profile not found."}
          </p>
        </div>
      </div>
    )
  }

  return (
    <>
      {/* Lightbox */}
      {lightbox && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/90"
          onClick={() => setLightbox(null)}
        >
          <button
            aria-label="Close"
            className="absolute right-4 top-4 rounded-full p-2 text-white/70 hover:text-white"
            onClick={() => setLightbox(null)}
          >
            <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={lightbox}
            alt=""
            className="max-h-[90vh] max-w-[90vw] rounded-lg object-contain"
            onClick={(e) => e.stopPropagation()}
          />
        </div>
      )}

      <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
        <div className="w-full max-w-lg space-y-6 px-4">

          {/* Header */}
          <div className="flex items-center justify-between">
            <h1 className="text-3xl font-bold text-white">Profile</h1>
            <button
              aria-label="Go back"
              onClick={() => router.back()}
              className="text-sm text-gray-400 hover:text-gray-200"
            >
              ← Back
            </button>
          </div>

          {error && (
            <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">{error}</p>
          )}

          {/* Identity card */}
          <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
            <div className="flex flex-col items-center gap-4 text-center">
              {/* Avatar */}
              {profile.avatar_url ? (
                <Image
                  src={profile.avatar_url}
                  alt=""
                  width={96}
                  height={96}
                  className="h-24 w-24 rounded-full object-cover ring-2 ring-gray-700"
                />
              ) : (
                <span className="flex h-24 w-24 items-center justify-center rounded-full bg-gray-700 text-3xl text-gray-300 ring-2 ring-gray-600">
                  {(profile.display_name || "?")[0].toUpperCase()}
                </span>
              )}

              <div>
                <h2 className="text-xl font-bold text-white">{profile.display_name}</h2>
                {profile.bio && (
                  <p className="mt-1 max-w-xs text-sm text-gray-400">{profile.bio}</p>
                )}
              </div>

              {/* Contact action */}
              {contactStatus === "loading" && (
                <span className="h-10 w-32 animate-pulse rounded-full bg-gray-800" />
              )}
              {contactStatus === "contact" && (
                <button
                  onClick={handleStartDM}
                  disabled={actionLoading}
                  className="rounded-full bg-indigo-600 px-5 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
                >
                  Message
                </button>
              )}
              {contactStatus === "sent" && (
                <span className="rounded-full bg-gray-800 px-5 py-2 text-sm text-gray-400 ring-1 ring-gray-700">
                  Request sent
                </span>
              )}
              {contactStatus === "incoming" && (
                <button
                  onClick={handleAccept}
                  disabled={actionLoading}
                  className="rounded-full bg-green-700 px-5 py-2 text-sm font-medium text-white hover:bg-green-600 disabled:opacity-50"
                >
                  Accept request
                </button>
              )}
              {contactStatus === "none" && (
                <button
                  onClick={handleAddContact}
                  disabled={actionLoading}
                  className="flex items-center gap-2 rounded-full bg-indigo-600 px-5 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
                >
                  <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M18 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0ZM3 19.235v-.11a6.375 6.375 0 0 1 12.75 0v.109A12.318 12.318 0 0 1 9.374 21c-2.331 0-4.512-.645-6.374-1.766Z" />
                  </svg>
                  Add contact
                </button>
              )}
            </div>
          </div>

          {/* Photos */}
          {profile.photos.length > 0 && (
            <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
              <h2 className="mb-4 text-lg font-semibold text-white">Photos</h2>
              <div className="grid grid-cols-3 gap-3">
                {profile.photos.map((photo) => (
                  <button
                    key={photo.id}
                    onClick={() => setLightbox(photo.url)}
                    className="relative aspect-square overflow-hidden rounded-lg bg-gray-800 transition-transform active:scale-95"
                  >
                    <Image
                      src={photo.url}
                      alt=""
                      fill
                      className="object-cover"
                      sizes="(max-width: 512px) 33vw, 170px"
                    />
                  </button>
                ))}
              </div>
            </div>
          )}

        </div>
      </div>
    </>
  )
}
