"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { useEffect, useRef, useState } from "react"

import { useUpload } from "@/hooks/useUpload"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Skeleton from "@/components/ui/Skeleton"
import PhotoGallery from "@/components/profile/PhotoGallery"

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
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-5 w-16" />
      </div>
      <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
        <div className="flex flex-col items-center gap-4">
          <Skeleton className="h-24 w-24 rounded-full" />
          <Skeleton className="h-4 w-28" />
        </div>
        <div className="mt-6 space-y-4">
          <div className="space-y-1.5">
            <Skeleton className="h-3.5 w-24" />
            <Skeleton className="h-10 w-full rounded-md" />
          </div>
          <div className="space-y-1.5">
            <Skeleton className="h-3.5 w-16" />
            <Skeleton className="h-20 w-full rounded-md" />
          </div>
          <Skeleton className="h-10 w-full rounded-md" />
        </div>
      </div>
    </div>
  )
}

function useAutoReset(value: string, setValue: (v: string) => void, delay = 10_000) {
  useEffect(() => {
    if (!value) return
    const id = setTimeout(() => setValue(""), delay)
    return () => clearTimeout(id)
  }, [value, setValue, delay])
}

export default function ProfilePage() {
  const { data: session, status } = useSession()
  const router = useRouter()

  const [profile, setProfile] = useState<Profile | null>(null)
  const [displayName, setDisplayName] = useState("")
  const [bio, setBio] = useState("")
  const [avatarURL, setAvatarURL] = useState("")
  const [formError, setFormError] = useState("")
  const [formSuccess, setFormSuccess] = useState("")
  const [avatarSuccess, setAvatarSuccess] = useState("")
  const [avatarError, setAvatarError] = useState("")
  const [galleryError, setGalleryError] = useState("")
  const [loadError, setLoadError] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [uploadingPhoto, setUploadingPhoto] = useState(false)

  useAutoReset(formSuccess, setFormSuccess)
  useAutoReset(avatarSuccess, setAvatarSuccess)
  useAutoReset(avatarError, setAvatarError)

  const avatarInputRef = useRef<HTMLInputElement>(null)
  const photoInputRef = useRef<HTMLInputElement>(null)
  const { upload, uploading: uploadingAvatar, error: uploadError } = useUpload(session?.accessToken)

  const token = session?.accessToken
  const isDirty = profile !== null && (displayName !== profile.display_name || bio !== profile.bio)

  useEffect(() => {
    if (status !== "authenticated" || !token) return
    fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      .then((res) => res.json())
      .then((data: Profile) => {
        setProfile(data)
        setDisplayName(data.display_name)
        setBio(data.bio)
        setAvatarURL(data.avatar_url)
      })
      .catch(() => setLoadError("Failed to load profile."))
      .finally(() => setLoading(false))
  }, [status, token])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFormError("")
    setFormSuccess("")
    setSaving(true)
    try {
      const res = await fetch(`${API_URL}/profiles/me`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ display_name: displayName, bio, avatar_url: avatarURL }),
      })
      if (!res.ok) { const data = await res.json(); setFormError(data.error ?? "Failed to update profile."); return }
      const updated: Profile = await res.json()
      setProfile(updated)
      setAvatarURL(updated.avatar_url)
      setAvatarSuccess("")
      setFormSuccess("Profile updated.")
    } finally {
      setSaving(false)
    }
  }

  async function handleAvatarChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setAvatarSuccess("")
    setAvatarError("")
    const result = await upload(file, "avatar")
    if (!result) return
    const res = await fetch(`${API_URL}/profiles/me/avatar`, {
      method: "PUT",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ avatar_url: result.url }),
    })
    if (!res.ok) { setAvatarError("Failed to save avatar."); return }
    setAvatarURL(result.url)
    setAvatarSuccess("Avatar updated.")
  }

  async function handleAddPhoto(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setGalleryError("")
    setUploadingPhoto(true)
    try {
      const result = await upload(file, "gallery")
      if (!result) return
      const res = await fetch(`${API_URL}/profiles/me/photos`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ url: result.url }),
      })
      if (!res.ok) { const data = await res.json(); setGalleryError(data.error ?? "Failed to add photo."); return }
      const photo: ProfilePhoto = await res.json()
      setProfile((prev) => prev ? { ...prev, photos: [...prev.photos, photo] } : prev)
    } finally {
      setUploadingPhoto(false)
      if (photoInputRef.current) photoInputRef.current.value = ""
    }
  }

  async function handleDeletePhoto(photoID: string) {
    setGalleryError("")
    const res = await fetch(`${API_URL}/profiles/me/photos/${photoID}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) { setGalleryError("Failed to delete photo."); return }
    setProfile((prev) => prev ? { ...prev, photos: prev.photos.filter((p) => p.id !== photoID) } : prev)
  }

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
        <ProfileSkeleton />
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
      <div className="w-full max-w-lg space-y-6 px-4">

        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold text-white">My Profile</h1>
          <Button variant="ghost" aria-label="Go to home" onClick={() => router.push("/")}>← Home</Button>
        </div>

        {loadError && (
          <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">{loadError}</p>
        )}

        {/* Profile card */}
        <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
          {/* Avatar upload — custom interactive widget */}
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
            {(uploadError || avatarError || avatarSuccess) && (
              <p className={`text-xs ${(uploadError || avatarError) ? "text-red-400" : "text-green-400"}`}>
                {uploadError || avatarError || avatarSuccess}
              </p>
            )}
          </div>

          <form onSubmit={handleSubmit} className="mt-6 space-y-4">
            <Input
              label="Display Name"
              id="displayName"
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              dirty={displayName !== (profile?.display_name ?? "")}
              required
            />

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
                className={`mt-1 block w-full rounded-md border bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:outline-none focus:ring-1 ${
                  bio !== (profile?.bio ?? "")
                    ? "border-orange-500 focus:border-orange-400 focus:ring-orange-400"
                    : "border-gray-700 focus:border-indigo-500 focus:ring-indigo-500"
                }`}
              />
            </div>

            {formError && <p className="text-sm text-red-400">{formError}</p>}
            {formSuccess && <p className="text-sm text-green-400">{formSuccess}</p>}

            <Button
              type="submit"
              variant={isDirty ? "warning" : "primary"}
              loading={saving}
              disabled={saving || uploadingAvatar || !isDirty}
              className="w-full focus:ring-offset-gray-900"
            >
              {saving ? "Saving…" : isDirty ? "Save changes" : "Save"}
            </Button>
          </form>
        </div>

        <PhotoGallery
          photos={profile?.photos ?? []}
          maxPhotos={MAX_PHOTOS}
          editable
          uploading={uploadingPhoto}
          onAdd={() => photoInputRef.current?.click()}
          onDelete={handleDeletePhoto}
          error={galleryError}
        />

        <input
          ref={photoInputRef}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={handleAddPhoto}
          className="hidden"
        />

      </div>
    </div>
  )
}
