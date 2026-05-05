"use client"

import Image from "next/image"
import { signOut, useSession } from "next-auth/react"
import { useEffect, useRef, useState, useCallback } from "react"
import { useTranslations } from "next-intl"

import { useRouter, Link } from "@/i18n/navigation"

import { useUpload } from "@/hooks/useUpload"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Skeleton from "@/components/ui/Skeleton"
import PhotoGallery from "@/components/profile/PhotoGallery"
import { computeCompleteness, type MissingField } from "@/lib/profileCompleteness"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface PhotonFeature {
  geometry: { coordinates: [number, number] }
  properties: { name?: string; city?: string; state?: string; country?: string }
}

const MAX_BIO = 280
const MAX_PHOTOS = 6
const MAX_INTERESTS = 20

const GENDER_OPTIONS = [
  "Male",
  "Female",
  "Trans male",
  "Trans female",
  "Non-binary",
  "Prefer not to say",
  "Custom",
]

const CUSTOM_GENDER = "Custom"

interface InterestSuggestion {
  name: string
  count: number
}

function useInterestSearch(query: string, token: string | undefined) {
  const [suggestions, setSuggestions] = useState<InterestSuggestion[]>([])

  const search = useCallback(async (q: string) => {
    if (!token || !q) return
    try {
      const res = await fetch(
        `${API_URL}/profiles/interests?q=${encodeURIComponent(q)}`,
        { headers: { Authorization: `Bearer ${token}` } },
      )
      if (!res.ok) return
      setSuggestions(await res.json())
    } catch {
      // silent fail
    }
  }, [token])

  useEffect(() => {
    const id = setTimeout(() => search(query), 300)
    return () => clearTimeout(id)
  }, [query, search])

  return { suggestions, clear: () => setSuggestions([]) }
}

interface LocationSuggestion {
  label: string
  lat: number
  lng: number
}

function useLocationSearch(query: string, enabled: boolean) {
  const [suggestions, setSuggestions] = useState<LocationSuggestion[]>([])
  const [searching, setSearching] = useState(false)

  const search = useCallback(async (q: string) => {
    if (q.length < 2) { setSuggestions([]); return }
    setSearching(true)
    try {
      const res = await fetch(
        `https://photon.komoot.io/api/?q=${encodeURIComponent(q)}&limit=5`,
        { headers: { "Accept-Language": navigator.language ?? "en" } },
      )
      if (!res.ok) return
      const data = await res.json()
      const seen = new Set<string>()
      const results = (data.features as PhotonFeature[]).flatMap((f) => {
        const p = f.properties
        const parts = [p.name, p.city ?? p.state, p.country].filter(Boolean)
        const label = parts.join(", ")
        const key = `${label}-${f.geometry.coordinates[1]},${f.geometry.coordinates[0]}`
        if (seen.has(key)) return []
        seen.add(key)
        return [{ label, lat: f.geometry.coordinates[1], lng: f.geometry.coordinates[0] }]
      })
      setSuggestions(results)
    } catch {
      setSuggestions([])
    } finally {
      setSearching(false)
    }
  }, [])

  useEffect(() => {
    if (!enabled) return
    const id = setTimeout(() => search(query), 350)
    return () => clearTimeout(id)
  }, [query, search, enabled])

  return { suggestions, searching, clear: () => setSuggestions([]) }
}

interface ProfilePhoto {
  id: string
  url: string
}

interface Profile {
  id: string
  user_id: string
  username: string
  display_name: string
  bio: string
  avatar_url: string
  date_of_birth?: string
  gender: string
  location_text: string
  latitude?: number
  longitude?: number
  interests: string[]
  photos: ProfilePhoto[]
}

type UsernameStatus = "idle" | "checking" | "available" | "taken" | "invalid"

function useUsernameAvailability(username: string, token: string | undefined, currentUsername: string) {
  const [status, setStatus] = useState<UsernameStatus>("idle")

  useEffect(() => {
    const id = setTimeout(async () => {
      if (!username || username === currentUsername) { setStatus("idle"); return }
      if (!/^[a-z0-9_]{3,30}$/.test(username)) { setStatus("invalid"); return }
      setStatus("checking")
      if (!token) return
      try {
        const res = await fetch(
          `${API_URL}/profiles/available?username=${encodeURIComponent(username)}`,
          { headers: { Authorization: `Bearer ${token}` } },
        )
        if (!res.ok) return
        const data = await res.json()
        setStatus(data.available ? "available" : "taken")
      } catch {
        setStatus("idle")
      }
    }, 400)
    return () => clearTimeout(id)
  }, [username, token, currentUsername])

  return status
}

function ProfileSkeleton() {
  return (
    <div className="w-full max-w-2xl space-y-6 px-4">
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
          <Skeleton className="h-10 w-full rounded-md" />
          <Skeleton className="h-10 w-full rounded-md" />
          <Skeleton className="h-10 w-full rounded-md" />
        </div>
      </div>
    </div>
  )
}

function effectiveGender(gender: string, genderOther: string): string {
  return gender === CUSTOM_GENDER ? genderOther : gender
}

function useAutoReset(value: string, setValue: (v: string) => void, delay = 10_000) {
  useEffect(() => {
    if (!value) return
    const id = setTimeout(() => setValue(""), delay)
    return () => clearTimeout(id)
  }, [value, setValue, delay])
}

export default function ProfilePage() {
  const t = useTranslations("profile")
  const tNav = useTranslations("nav")
  const { data: session, status } = useSession()
  const router = useRouter()

  async function handleSignOut() {
    const token = session?.accessToken
    if (token) {
      try {
        await fetch(`${API_URL}/presence/heartbeat`, {
          method: "DELETE",
          headers: { Authorization: `Bearer ${token}` },
        })
      } catch { /* non-critical */ }
    }
    if (session?.refreshToken) {
      try {
        await fetch(`${API_URL}/auth/logout`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: session.refreshToken }),
        })
      } catch { /* non-critical */ }
    }
    await signOut()
  }

  const [profile, setProfile] = useState<Profile | null>(null)
  const [username, setUsername] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [bio, setBio] = useState("")
  const [avatarURL, setAvatarURL] = useState("")
  const [dateOfBirth, setDateOfBirth] = useState("")
  const [gender, setGender] = useState("")
  const [genderOther, setGenderOther] = useState("")
  const [locationText, setLocationText] = useState("")
  const [locationLat, setLocationLat] = useState<number | null>(null)
  const [locationLng, setLocationLng] = useState<number | null>(null)
  const [interests, setInterests] = useState<string[]>([])
  const [locationQuery, setLocationQuery] = useState("")
  const [locationEdited, setLocationEdited] = useState(false)
  const [interestInput, setInterestInput] = useState("")
  const [interestFocused, setInterestFocused] = useState(false)
  const locationRef = useRef<HTMLDivElement>(null)
  const interestRef = useRef<HTMLDivElement>(null)
  const [formError, setFormError] = useState("")
  const [formSuccess, setFormSuccess] = useState("")
  const [avatarSuccess, setAvatarSuccess] = useState("")
  const [avatarError, setAvatarError] = useState("")
  const [galleryError, setGalleryError] = useState("")
  const [loadError, setLoadError] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [uploadingPhoto, setUploadingPhoto] = useState(false)

  const token = session?.accessToken

  const usernameStatus = useUsernameAvailability(username, token, profile?.username ?? "")
  const { suggestions: locationSuggestions, searching: locationSearching, clear: clearLocationSuggestions } = useLocationSearch(locationQuery, locationEdited)
  const { suggestions: interestSuggestions, clear: clearInterestSuggestions } = useInterestSearch(interestFocused ? interestInput : "", token)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (locationRef.current && !locationRef.current.contains(e.target as Node)) {
        clearLocationSuggestions()
      }
      if (interestRef.current && !interestRef.current.contains(e.target as Node)) {
        setInterestFocused(false)
        clearInterestSuggestions()
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [clearLocationSuggestions, clearInterestSuggestions])

  useAutoReset(formSuccess, setFormSuccess)
  useAutoReset(avatarSuccess, setAvatarSuccess)
  useAutoReset(avatarError, setAvatarError)

  const avatarInputRef = useRef<HTMLInputElement>(null)
  const photoInputRef = useRef<HTMLInputElement>(null)
  const { upload, uploading: uploadingAvatar, error: uploadError } = useUpload(session?.accessToken)

  const isDirty = profile !== null && (
    username !== (profile.username ?? "") ||
    displayName !== profile.display_name ||
    bio !== profile.bio ||
    (dateOfBirth ?? "") !== (profile.date_of_birth ?? "") ||
    effectiveGender(gender, genderOther) !== profile.gender ||
    locationText !== profile.location_text ||
    JSON.stringify(interests) !== JSON.stringify(profile.interests)
  )

  useEffect(() => {
    if (status !== "authenticated" || !token) return
    fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      .then((res) => (res.ok ? res.json() : Promise.reject(res.status)))
      .then((data: Profile) => {
        setProfile(data)
        setUsername(data.username ?? "")
        setDisplayName(data.display_name ?? "")
        setBio(data.bio ?? "")
        setAvatarURL(data.avatar_url)
        setDateOfBirth(data.date_of_birth ?? "")
        if (data.gender && !GENDER_OPTIONS.includes(data.gender)) {
          setGender(CUSTOM_GENDER)
          setGenderOther(data.gender)
        } else {
          setGender(data.gender ?? "")
        }
        setLocationText(data.location_text ?? "")
        setLocationQuery(data.location_text ?? "")
        setLocationLat(data.latitude ?? null)
        setLocationLng(data.longitude ?? null)
        setInterests(data.interests ?? [])
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
      const body: Record<string, unknown> = {
        username,
        display_name: displayName,
        bio,
        avatar_url: avatarURL,
        gender: effectiveGender(gender, genderOther),
        location_text: locationText,
        latitude: locationLat,
        longitude: locationLng,
        interests,
        date_of_birth: dateOfBirth || undefined,
      }

      const res = await fetch(`${API_URL}/profiles/me`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(body),
      })
      if (!res.ok) { const data = await res.json(); setFormError(data.error ?? "Failed to update profile."); return }
      const updated: Profile = await res.json()
      setProfile(updated)
      setUsername(updated.username ?? "")
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

  function addInterest(name: string) {
    const tag = name.trim().toLowerCase().replace(/,/g, "")
    if (!tag || interests.includes(tag) || interests.length >= MAX_INTERESTS) return
    setInterests((prev) => [...prev, tag])
    setInterestInput("")
    clearInterestSuggestions()
  }

  function handleAddInterest(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key !== "Enter" && e.key !== ",") return
    e.preventDefault()
    addInterest(interestInput)
  }

  function handleRemoveInterest(tag: string) {
    setInterests((prev) => prev.filter((t) => t !== tag))
  }

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
        <ProfileSkeleton />
      </div>
    )
  }

  const profileIncomplete = profile !== null && (!profile.username || !profile.date_of_birth)

  const completeness = profile ? computeCompleteness({
    avatar_url: profile.avatar_url,
    bio: profile.bio,
    interests: profile.interests,
    date_of_birth: profile.date_of_birth,
    location_text: profile.location_text,
  }) : null

  const ONBOARDING_STEP: Record<MissingField, string> = {
    avatar: "/onboarding/photo",
    bio: "/onboarding/bio",
    interests: "/onboarding/interests",
    birthdate: "/profile",
    location: "/onboarding/location",
  }

  return (
    <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
      <div className="w-full max-w-2xl space-y-6 px-4">

        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold text-white">{t("title")}</h1>
          <div className="flex items-center gap-2">
            {profile?.username && (
              <Button variant="ghost" size="sm" onClick={() => router.push(`/profile/${profile.username}`)}>
                {t("previewAsVisitor")}
              </Button>
            )}
            {/* Settings + Logout — mobile only; desktop uses the sidebar */}
            <Link
              href="/settings"
              aria-label={tNav("settings")}
              className="lg:hidden cursor-pointer rounded-md p-3 text-gray-400 transition-colors hover:bg-gray-800 hover:text-white focus:outline-none focus:ring-2 focus:ring-brand-hover"
            >
              <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <circle cx="12" cy="12" r="3" />
                <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
              </svg>
            </Link>
            <button
              type="button"
              onClick={handleSignOut}
              aria-label={tNav("logOut")}
              className="lg:hidden cursor-pointer rounded-md p-3 text-gray-400 transition-colors hover:bg-gray-800 hover:text-red-400 focus:outline-none focus:ring-2 focus:ring-brand-hover"
            >
              <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                <polyline points="16 17 21 12 16 7" />
                <line x1="21" y1="12" x2="9" y2="12" />
              </svg>
            </button>
          </div>
        </div>

        {profileIncomplete && (
          <div className="rounded-lg bg-amber-950 p-4 ring-1 ring-amber-700">
            <p className="text-sm font-medium text-amber-300">Complete your profile to use Circl</p>
            <p className="mt-1 text-xs text-amber-400">
              {!profile.username && !profile.date_of_birth
                ? "Set your username and date of birth below."
                : !profile.username
                ? "Set your username below."
                : "Set your date of birth below."}
            </p>
          </div>
        )}

        {/* Profile completeness card */}
        {completeness && completeness.percent < 100 && (
          <div className="rounded-lg bg-gray-900 p-5 ring-1 ring-gray-800 space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-sm font-semibold text-white">{t("completeness.title")}</p>
              <span className="text-sm font-bold text-brand-accent">{completeness.percent}%</span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-gray-800">
              <div
                className="h-full rounded-full bg-brand-accent transition-all duration-500"
                style={{ width: `${completeness.percent}%` }}
              />
            </div>
            <ul className="space-y-2">
              {completeness.missing.map((field) => (
                <li key={field}>
                  <button
                    type="button"
                    onClick={() => router.push(ONBOARDING_STEP[field])}
                    className="flex w-full items-center gap-2 text-left text-sm text-gray-400 hover:text-white"
                  >
                    <span className="h-1.5 w-1.5 rounded-full bg-gray-600 shrink-0" />
                    {t(`completeness.${field}`)}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}

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
              className="group relative h-24 w-24 overflow-hidden rounded-full bg-gray-800 ring-2 ring-gray-700 transition-all hover:ring-brand-hover focus:outline-none focus:ring-brand-hover"
            >
              {avatarURL ? (
                <Image src={avatarURL} alt="Your avatar" width={96} height={96} className="h-full w-full object-cover" />
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
            <div>
              <div className="flex items-center justify-between">
                <label htmlFor="username" className="block text-sm font-medium text-gray-300">
                  Username <span className="text-red-400">*</span>
                </label>
                {!profile?.username && username && (
                  <span className={`text-xs ${
                    usernameStatus === "available" ? "text-green-400" :
                    usernameStatus === "taken" ? "text-red-400" :
                    usernameStatus === "invalid" ? "text-yellow-400" :
                    "text-gray-500"
                  }`}>
                    {usernameStatus === "checking" ? "Checking…" :
                     usernameStatus === "available" ? <><svg className="mr-0.5 inline h-3 w-3" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>Available</> :
                     usernameStatus === "taken" ? <><svg className="mr-0.5 inline h-3 w-3" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>Taken</> :
                     usernameStatus === "invalid" ? "3–30 chars, lowercase, digits, _" : ""}
                  </span>
                )}
              </div>
              <div className="relative mt-1">
                <span className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-gray-500">@</span>
                <input
                  id="username"
                  type="text"
                  value={username}
                  onChange={profile?.username ? undefined : (e) => setUsername(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ""))}
                  readOnly={!!profile?.username}
                  maxLength={30}
                  placeholder="your_username"
                  required
                  className={`block w-full rounded-md border bg-gray-800 pl-7 pr-3 py-2 placeholder-gray-500 shadow-sm focus:outline-none focus:ring-1 ${
                    profile?.username
                      ? "border-gray-700 text-gray-400 cursor-not-allowed"
                      : username !== (profile?.username ?? "")
                      ? "border-orange-500 text-white focus:border-orange-400 focus:ring-orange-400"
                      : "border-gray-700 text-white focus:border-brand-hover focus:ring-brand-hover"
                  }`}
                />
              </div>
              {profile?.username && (
                <p className="mt-1 text-xs text-gray-500">Username cannot be changed once set.</p>
              )}
            </div>

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
                    : "border-gray-700 focus:border-brand-hover focus:ring-brand-hover"
                }`}
              />
            </div>

            <Input
              label="Date of birth"
              id="dateOfBirth"
              type="date"
              value={dateOfBirth}
              onChange={(e) => setDateOfBirth(e.target.value)}
              dirty={(dateOfBirth ?? "") !== (profile?.date_of_birth ?? "")}
              helper="You must be at least 18 years old."
            />

            <div>
              <label htmlFor="gender" className="block text-sm font-medium text-gray-300">Gender</label>
              <select
                id="gender"
                value={gender}
                onChange={(e) => { setGender(e.target.value); if (e.target.value !== CUSTOM_GENDER) setGenderOther("") }}
                className={`mt-1 block w-full rounded-md border bg-gray-800 px-3 py-2 text-white shadow-sm focus:outline-none focus:ring-1 ${
                  effectiveGender(gender, genderOther) !== (profile?.gender ?? "")
                    ? "border-orange-500 focus:border-orange-400 focus:ring-orange-400"
                    : "border-gray-700 focus:border-brand-hover focus:ring-brand-hover"
                }`}
              >
                <option value="">Prefer not to say</option>
                {GENDER_OPTIONS.map((opt) => (
                  <option key={opt} value={opt}>{opt}</option>
                ))}
              </select>
              {gender === CUSTOM_GENDER && (
                <input
                  type="text"
                  value={genderOther}
                  onChange={(e) => setGenderOther(e.target.value)}
                  placeholder="Describe your gender…"
                  maxLength={50}
                  className="mt-2 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"
                />
              )}
            </div>

            <div ref={locationRef} className="relative">
              <label htmlFor="location" className="block text-sm font-medium text-gray-300">Location</label>
              <div className="relative mt-1">
                <input
                  id="location"
                  type="text"
                  value={locationQuery}
                  onChange={(e) => { setLocationQuery(e.target.value); setLocationText(e.target.value); setLocationLat(null); setLocationLng(null); setLocationEdited(true) }}
                  placeholder="e.g. Paris, France"
                  autoComplete="off"
                  className={`block w-full rounded-md border bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:outline-none focus:ring-1 ${
                    locationText !== (profile?.location_text ?? "")
                      ? "border-orange-500 focus:border-orange-400 focus:ring-orange-400"
                      : "border-gray-700 focus:border-brand-hover focus:ring-brand-hover"
                  }`}
                />
                {locationSearching && (
                  <span className="absolute right-3 top-2.5 text-xs text-gray-500">searching…</span>
                )}
              </div>
              {locationSuggestions.length > 0 && (
                <ul className="absolute z-10 mt-1 w-full rounded-md border border-gray-700 bg-gray-800 shadow-lg">
                  {locationSuggestions.map((s) => (
                    <li key={`${s.label}-${s.lat},${s.lng}`}>
                      <button
                        type="button"
                        onClick={() => {
                          setLocationText(s.label)
                          setLocationQuery(s.label)
                          setLocationLat(s.lat)
                          setLocationLng(s.lng)
                          setLocationEdited(false)
                          clearLocationSuggestions()
                        }}
                        className="w-full px-3 py-2 text-left text-sm text-gray-200 hover:bg-gray-700 focus:bg-gray-700 focus:outline-none"
                      >
                        {s.label}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            {/* Interests tag input */}
            <div ref={interestRef} className="relative">
              <div className="flex items-center justify-between">
                <label htmlFor="interestInput" className="block text-sm font-medium text-gray-300">Interests</label>
                <span className={`text-xs ${interests.length >= MAX_INTERESTS ? "text-red-400" : "text-gray-500"}`}>
                  {interests.length}/{MAX_INTERESTS}
                </span>
              </div>
              <input
                id="interestInput"
                type="text"
                value={interestInput}
                onChange={(e) => setInterestInput(e.target.value)}
                onKeyDown={handleAddInterest}
                onFocus={() => setInterestFocused(true)}
                placeholder="Type and press Enter to add"
                disabled={interests.length >= MAX_INTERESTS}
                autoComplete="off"
                className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover disabled:opacity-50"
              />
              {interestSuggestions.length > 0 && (
                <ul className="absolute z-10 mt-1 w-full rounded-md border border-gray-700 bg-gray-800 shadow-lg">
                  {interestSuggestions
                    .filter((s) => !interests.includes(s.name))
                    .map((s) => (
                      <li key={s.name}>
                        <button
                          type="button"
                          onClick={() => addInterest(s.name)}
                          className="flex w-full items-center justify-between px-3 py-2 text-left text-sm text-gray-200 hover:bg-gray-700 focus:bg-gray-700 focus:outline-none"
                        >
                          <span>{s.name}</span>
                          <span className="text-xs text-gray-500">{s.count}</span>
                        </button>
                      </li>
                    ))}
                </ul>
              )}
              {interests.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-2">
                  {interests.map((tag) => (
                    <span
                      key={tag}
                      className="flex items-center gap-1 rounded-full bg-brand-wash/60 px-3 py-1 text-xs text-brand-subtle ring-1 ring-brand-strong"
                    >
                      {tag}
                      <button
                        type="button"
                        onClick={() => handleRemoveInterest(tag)}
                        aria-label={`Remove ${tag}`}
                        className="ml-1 text-brand-muted hover:text-white"
                      >
                        ×
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>

            {formError && <p className="text-sm text-red-400">{formError}</p>}
            {formSuccess && <p className="text-sm text-green-400">{formSuccess}</p>}

            <Button
              type="submit"
              variant={isDirty ? "warning" : "primary"}
              loading={saving}
              disabled={saving || uploadingAvatar || !isDirty || (!profile?.username && (usernameStatus === "checking" || usernameStatus === "taken" || usernameStatus === "invalid"))}
              className="w-full focus:ring-offset-gray-900"
            >
              {saving ? t("saving") : isDirty ? t("save") : t("save")}
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
