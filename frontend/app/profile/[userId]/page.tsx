"use client"

import { useSession } from "next-auth/react"
import { useParams, useRouter } from "next/navigation"
import { useEffect, useState } from "react"

import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"
import PhotoGallery from "@/components/profile/PhotoGallery"
import ProfileHeader, { type ContactStatus } from "@/components/profile/ProfileHeader"
import Lightbox from "@/components/chat/Lightbox"

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
  date_of_birth?: string
  gender: string
  location_text: string
  interests: string[]
  photos: ProfilePhoto[]
}

interface SentRequest  { contact_id: string; user_id: string }
interface AcceptedContact { contact_id: string; user_id: string }
interface PendingRequest  { contact_id: string; user_id: string }

function ProfileSkeleton() {
  return (
    <div className="w-full max-w-lg space-y-6 px-4">
      <div className="flex items-center justify-between">
        <Skeleton className="h-8 w-24" />
        <Skeleton className="h-5 w-16" />
      </div>
      <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
        <div className="flex flex-col items-center gap-4">
          <Skeleton className="h-24 w-24 rounded-full" />
          <Skeleton className="h-5 w-36" />
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-10 w-32 rounded-full" />
        </div>
      </div>
      <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
        <div className="grid grid-cols-3 gap-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="aspect-square rounded-lg" />
          ))}
        </div>
      </div>
    </div>
  )
}

function formatAge(dateOfBirth?: string): string | null {
  if (!dateOfBirth) return null
  const dob = new Date(dateOfBirth)
  const today = new Date()
  let age = today.getFullYear() - dob.getFullYear()
  const m = today.getMonth() - dob.getMonth()
  if (m < 0 || (m === 0 && today.getDate() < dob.getDate())) age--
  return `${age}`
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
    if (myID && userId && myID === userId) router.replace("/profile")
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
        if (profileRes.status === 404) { setError("Profile not found."); return }
        if (!profileRes.ok) throw new Error("Failed to load profile.")

        const [prof, contacts, sent, pending]: [PublicProfile, AcceptedContact[], SentRequest[], PendingRequest[]] =
          await Promise.all([
            profileRes.json(),
            contactsRes.ok ? contactsRes.json() : [],
            sentRes.ok ? sentRes.json() : [],
            pendingRes.ok ? pendingRes.json() : [],
          ])

        setProfile(prof)
        const accepted = contacts.find((c) => c.user_id === userId)
        if (accepted) { setContactStatus("contact"); setContactId(accepted.contact_id); return }
        const sentEntry = sent.find((s) => s.user_id === userId)
        if (sentEntry) { setContactStatus("sent"); setContactId(sentEntry.contact_id); return }
        const incomingEntry = pending.find((p) => p.user_id === userId)
        if (incomingEntry) { setContactStatus("incoming"); setContactId(incomingEntry.contact_id); return }
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
      if (res.status === 409) { setContactStatus("sent"); return }
      if (!res.ok) { setError("Failed to send contact request."); return }
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
      if (!res.ok) { setError("Failed to accept contact."); return }
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
      if (!res.ok) { setError("Failed to open conversation."); return }
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
            <Button variant="ghost" aria-label="Go back" onClick={() => router.back()}>← Back</Button>
          </div>
          <p className="mt-6 rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">
            {error || "Profile not found."}
          </p>
        </div>
      </div>
    )
  }

  const age = formatAge(profile.date_of_birth)

  return (
    <>
      {lightbox && (
        <Lightbox url={lightbox} type="image" onClose={() => setLightbox(null)} />
      )}

      <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
        <div className="w-full max-w-lg space-y-6 px-4">

          <div className="flex items-center justify-between">
            <h1 className="text-3xl font-bold text-white">Profile</h1>
            <Button variant="ghost" aria-label="Go back" onClick={() => router.back()}>← Back</Button>
          </div>

          {error && (
            <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">{error}</p>
          )}

          <ProfileHeader
            profile={profile}
            contactStatus={contactStatus}
            actionLoading={actionLoading}
            onAddContact={handleAddContact}
            onAccept={handleAccept}
            onStartDM={handleStartDM}
          />

          {/* Extended profile details */}
          {(age || profile.gender || profile.location_text || profile.interests?.length > 0) && (
            <div className="rounded-lg bg-gray-900 p-5 shadow-xl ring-1 ring-gray-800 space-y-3">
              {(age || profile.gender || profile.location_text) && (
                <div className="flex flex-wrap gap-3 text-sm text-gray-400">
                  {age && profile.gender ? (
                    <span>{age} · {profile.gender}</span>
                  ) : age ? (
                    <span>{age} years old</span>
                  ) : profile.gender ? (
                    <span>{profile.gender}</span>
                  ) : null}
                  {profile.location_text && (
                    <span className="flex items-center gap-1">
                      <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                          d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                          d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
                      </svg>
                      {profile.location_text}
                    </span>
                  )}
                </div>
              )}
              {profile.interests?.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {profile.interests.map((tag) => (
                    <span
                      key={tag}
                      className="rounded-full bg-indigo-900/50 px-3 py-1 text-xs text-indigo-300 ring-1 ring-indigo-700/60"
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}

          <PhotoGallery
            photos={profile.photos}
            onPhotoClick={setLightbox}
          />

        </div>
      </div>
    </>
  )
}
