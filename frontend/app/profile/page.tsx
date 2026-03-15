"use client"

import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { useEffect, useRef, useState } from "react"

import { useUpload } from "@/hooks/useUpload"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface Profile {
  id: string
  user_id: string
  display_name: string
  bio: string
  avatar_url: string
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
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { upload, uploading, error: uploadError } = useUpload(session?.accessToken)

  useEffect(() => {
    if (status === "unauthenticated") {
      router.push("/login")
    }
  }, [status, router])

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) return

    fetch(`${API_URL}/profiles/me`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
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
  }, [status, session])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError("")
    setSuccess("")

    const res = await fetch(`${API_URL}/profiles/me`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${session?.accessToken}`,
      },
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
    setSuccess("Profile updated successfully.")
  }

  async function handleAvatarChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setError("")
    setSuccess("")
    const result = await upload(file, "avatar")
    if (result) {
      setAvatarURL(result.url)
      setSuccess("Avatar uploaded. Click Save to keep it.")
    }
  }

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950">
        <p className="text-gray-400">Loading...</p>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-950">
      <div className="w-full max-w-md space-y-6 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div className="flex flex-col items-center gap-3">
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            className="group relative h-24 w-24 rounded-full bg-gray-800 ring-2 ring-gray-700 hover:ring-indigo-500 focus:outline-none focus:ring-indigo-500 overflow-hidden"
          >
            {avatarURL ? (
              <img src={avatarURL} alt="Avatar" className="h-full w-full object-cover" />
            ) : (
              <span className="text-3xl text-gray-500 group-hover:text-gray-300">
                {displayName ? displayName[0].toUpperCase() : "?"}
              </span>
            )}
            <span className="absolute inset-0 flex items-center justify-center bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity text-xs text-white">
              {uploading ? "Uploading..." : "Change"}
            </span>
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            onChange={handleAvatarChange}
            className="hidden"
          />
          <h1 className="text-center text-3xl font-bold text-white">My Profile</h1>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
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
            <label htmlFor="bio" className="block text-sm font-medium text-gray-300">
              Bio
            </label>
            <textarea
              id="bio"
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              rows={3}
              className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
          </div>

          {(error || uploadError) && <p className="text-sm text-red-400">{error || uploadError}</p>}
          {success && <p className="text-sm text-green-400">{success}</p>}

          <button
            type="submit"
            className="w-full rounded-md bg-indigo-600 px-4 py-2 text-white hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900"
          >
            Save
          </button>
        </form>

        <button
          onClick={() => router.push("/")}
          className="w-full rounded-md border border-gray-700 px-4 py-2 text-gray-300 hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900"
        >
          Back to Home
        </button>
      </div>
    </div>
  )
}
