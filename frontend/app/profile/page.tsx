"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { useEffect, useRef, useState } from "react"

import { useUpload } from "@/hooks/useUpload"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const MAX_BIO = 280
const MAX_PHOTOS = 6

interface ProfilePhoto {
  id: string
  url: string
}

interface Profile {
  id: string
  user_id: string
  display_name: string
  bio: string
  avatar_url: string
  photos: ProfilePhoto[]
}

function ProfileSkeleton() {
  return (
    <div className="w-full max-w-lg space-y-6 px-4">
      <div className="flex items-center justify-between">
        <span className="h-8 w-32 animate-pulse rounded bg-gray-800" />
        <span className="h-5 w-16 animate-pulse rounded bg-gray-800" />
      </div>
      <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
        <div className="flex flex-col items-center gap-4">
          <span className="h-24 w-24 animate-pulse rounded-full bg-gray-800" />
          <span className="h-4 w-28 animate-pulse rounded bg-gray-800" />
        </div>
        <div className="mt-6 space-y-4">
          <div className="space-y-1.5">
            <span className="block h-3.5 w-24 animate-pulse rounded bg-gray-800" />
            <span className="block h-10 w-full animate-pulse rounded-md bg-gray-800" />
          </div>
          <div className="space-y-1.5">
            <span className="block h-3.5 w-16 animate-pulse rounded bg-gray-800" />
            <span className="block h-20 w-full animate-pulse rounded-md bg-gray-800" />
          </div>
          <span className="block h-10 w-full animate-pulse rounded-md bg-gray-800" />
        </div>
      </div>
    </div>
  )
}

export default function ProfilePage() {
  const { data: session, status } = useSession()
  const router = useRouter()

  const [profile, setProfile] = useState<Profile | null>(null)
  const [displayName, setDisplayName] = useState("")
  const [bio, setBio] = useState("")
  const [avatarURL, setAvatarURL] = useState("")
  const [error, setError] = useState("")
  const [success, setSuccess] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [uploadingPhoto, setUploadingPhoto] = useState(false)

  const avatarInputRef = useRef<HTMLInputElement>(null)
  const photoInputRef = useRef<HTMLInputElement>(null)
  const { upload, uploading: uploadingAvatar, error: uploadError } = useUpload(session?.accessToken)

  const token = session?.accessToken

  useEffect(() => {
    if (status === "unauthenticated") router.push("/login")
  }, [status, router])

  useEffect(() => {
    if (status !== "authenticated" || !token) return

    fetch(`${API_URL}/profiles/me`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data: Profile) => {
        setProfile(data)
        setDisplayName(data.display_name)
        setBio(data.bio)
        setAvatarURL(data.avatar_url)
      })
      .catch(() => setError("Failed to load profile."))
      .finally(() => setLoading(false))
  }, [status, token])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError("")
    setSuccess("")
    setSaving(true)

    try {
      const res = await fetch(`${API_URL}/profiles/me`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ display_name: displayName, bio, avatar_url: avatarURL }),
      })
      if (!res.ok) {
        const data = await res.json()
        setError(data.error ?? "Failed to update profile.")
        return
      }
      const updated: Profile = await res.json()
      setProfile(updated)
      setAvatarURL(updated.avatar_url)
      setSuccess("Profile updated.")
    } finally {
      setSaving(false)
    }
  }

  async function handleAvatarChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setError("")
    setSuccess("")
    const result = await upload(file, "avatar")
    if (result) {
      setAvatarURL(result.url)
      setSuccess("Avatar ready — click Save to apply.")
    }
  }

  async function handleAddPhoto(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setError("")
    setUploadingPhoto(true)
    try {
      const result = await upload(file, "gallery")
      if (!result) return
      const res = await fetch(`${API_URL}/profiles/me/photos`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ url: result.url }),
      })
      if (!res.ok) {
        const data = await res.json()
        setError(data.error ?? "Failed to add photo.")
        return
      }
      const photo: ProfilePhoto = await res.json()
      setProfile((prev) => prev ? { ...prev, photos: [...prev.photos, photo] } : prev)
    } finally {
      setUploadingPhoto(false)
      if (photoInputRef.current) photoInputRef.current.value = ""
    }
  }

  async function handleDeletePhoto(photoID: string) {
    setError("")
    const res = await fetch(`${API_URL}/profiles/me/photos/${photoID}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) {
      setError("Failed to delete photo.")
      return
    }
    setProfile((prev) => prev ? { ...prev, photos: prev.photos.filter((p) => p.id !== photoID) } : prev)
  }

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
        <ProfileSkeleton />
      </div>
    )
  }

  const photos = profile?.photos ?? []
  const canAddPhoto = photos.length < MAX_PHOTOS && !uploadingPhoto

  return (
    <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
      <div className="w-full max-w-lg space-y-6 px-4">

        {/* Header */}
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold text-white">My Profile</h1>
          <button
            aria-label="Go to home"
            onClick={() => router.push("/")}
            className="text-sm text-gray-400 hover:text-gray-200"
          >
            ← Home
          </button>
        </div>

        {(error || uploadError) && (
          <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">
            {error || uploadError}
          </p>
        )}
        {success && (
          <p className="rounded-md bg-green-950 p-3 text-sm text-green-400 ring-1 ring-green-900">
            {success}
          </p>
        )}

        {/* Profile card */}
        <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
          {/* Avatar */}
          <div className="flex flex-col items-center gap-3">
            <button
              type="button"
              aria-label="Change avatar"
              onClick={() => avatarInputRef.current?.click()}
              disabled={uploadingAvatar}
              className="group relative h-24 w-24 overflow-hidden rounded-full bg-gray-800 ring-2 ring-gray-700 transition-all hover:ring-indigo-500 focus:outline-none focus:ring-indigo-500"
            >
              {avatarURL ? (
                <Image src={avatarURL} alt="" width={96} height={96} className="h-full w-full object-cover" />
              ) : (
                <span className="flex h-full w-full items-center justify-center text-3xl text-gray-500 group-hover:text-gray-300">
                  {displayName ? displayName[0].toUpperCase() : "?"}
                </span>
              )}
              <span className="absolute inset-0 flex items-center justify-center bg-black/50 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100">
                {uploadingAvatar ? "Uploading…" : "Change"}
              </span>
            </button>
            <input ref={avatarInputRef} type="file" accept="image/jpeg,image/png,image/webp" onChange={handleAvatarChange} className="hidden" />
            <p className="text-sm text-gray-500">{session?.user?.email}</p>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="mt-6 space-y-4">
            <div>
              <label htmlFor="displayName" className="block text-sm font-medium text-gray-300">
                Display Name
              </label>
              <input
                id="displayName"
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                required
              />
            </div>

            <div>
              <div className="flex items-center justify-between">
                <label htmlFor="bio" className="block text-sm font-medium text-gray-300">Bio</label>
                <span className={`text-xs ${bio.length > MAX_BIO ? "text-red-400" : "text-gray-500"}`}>
                  {bio.length}/{MAX_BIO}
                </span>
              </div>
              <textarea
                id="bio"
                value={bio}
                onChange={(e) => setBio(e.target.value)}
                rows={3}
                maxLength={MAX_BIO}
                className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>

            <button
              type="submit"
              disabled={saving || uploadingAvatar}
              className="flex w-full items-center justify-center gap-2 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900"
            >
              {saving && (
                <svg className="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
              )}
              {saving ? "Saving…" : "Save"}
            </button>
          </form>
        </div>

        {/* Photos */}
        <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-white">Photos</h2>
            <span className="text-xs text-gray-500">{photos.length}/{MAX_PHOTOS}</span>
          </div>

          <div className="grid grid-cols-3 gap-3">
            {photos.map((photo) => (
              <div key={photo.id} className="group relative aspect-square overflow-hidden rounded-lg bg-gray-800">
                <Image
                  src={photo.url}
                  alt=""
                  fill
                  className="object-cover"
                  sizes="(max-width: 512px) 33vw, 170px"
                />
                <button
                  aria-label="Delete photo"
                  onClick={() => handleDeletePhoto(photo.id)}
                  className="absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity hover:bg-black/80 group-hover:opacity-100 focus:opacity-100"
                >
                  <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            ))}

            {canAddPhoto && (
              <button
                type="button"
                onClick={() => photoInputRef.current?.click()}
                disabled={uploadingPhoto}
                className="flex aspect-square items-center justify-center rounded-lg border-2 border-dashed border-gray-700 text-gray-600 transition-colors hover:border-indigo-600 hover:text-indigo-500"
              >
                {uploadingPhoto ? (
                  <svg className="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>
                ) : (
                  <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                  </svg>
                )}
                <span className="sr-only">Add photo</span>
              </button>
            )}
          </div>

          <input
            ref={photoInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            onChange={handleAddPhoto}
            className="hidden"
          />
        </div>

      </div>
    </div>
  )
}
