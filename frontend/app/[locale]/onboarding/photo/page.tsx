"use client"

import Image from "next/image"
import { useRef, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"
import { useUpload } from "@/hooks/useUpload"
import { useProfileContext } from "@/contexts/ProfileContext"
import { nextStepAfter } from "@/lib/onboardingSteps"
import Button from "@/components/ui/Button"
import UploadRejectionModal from "@/components/upload/UploadRejectionModal"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function OnboardingPhotoPage() {
  const { data: session } = useSession()
  const router = useRouter()
  const t = useTranslations("onboarding")
  const fileRef = useRef<HTMLInputElement>(null)
  const { profile, refresh } = useProfileContext()

  const existing = profile?.avatar_url ?? ""
  const [preview, setPreview] = useState<string | null>(existing || null)
  const [avatarUrl, setAvatarUrl] = useState<string | null>(existing || null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")

  const { upload, uploading, rejection, clearRejection } = useUpload(session?.accessToken)
  const token = session?.accessToken

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setError("")
    const result = await upload(file, "avatar")
    if (!result) return
    setPreview(result.url)
    setAvatarUrl(result.url)
  }

  async function handleContinue() {
    if (!token) return
    const next = nextStepAfter(profile!, "photo")

    if (avatarUrl && avatarUrl !== existing) {
      setSaving(true)
      setError("")
      try {
        const res = await fetch(`${API_URL}/profiles/me/avatar`, {
          method: "PUT",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({ avatar_url: avatarUrl }),
        })
        if (!res.ok) { setError(t("saveError")); return }
      } finally {
        setSaving(false)
      }
    }

    if (next === "/") {
      setSaving(true)
      try {
        await fetch(`${API_URL}/profiles/me`, {
          method: "PUT",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({ mark_onboarded: true }),
        })
        await refresh()
        router.replace("/")
      } finally {
        setSaving(false)
      }
    } else {
      router.push(next)
    }
  }

  const hasNewPhoto = !!avatarUrl && avatarUrl !== existing

  return (
    <div className="flex flex-col gap-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold text-white">{t("photo.title")}</h1>
        <p className="text-sm text-gray-400">{t("photo.subtitle")}</p>
      </div>

      <div className="flex flex-col items-center gap-4">
        <button
          type="button"
          onClick={() => fileRef.current?.click()}
          disabled={uploading}
          className="group relative h-36 w-36 overflow-hidden rounded-full bg-gray-800 ring-2 ring-gray-700 transition-all hover:ring-brand-accent focus:outline-none focus:ring-brand-accent"
          aria-label={t("photo.choosePhoto")}
        >
          {preview ? (
            <Image src={preview} alt="Profile photo preview" fill className="object-cover" />
          ) : (
            <span className="flex h-full w-full flex-col items-center justify-center gap-1 text-gray-500 group-hover:text-gray-300">
              <svg className="h-10 w-10" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M6.827 6.175A2.31 2.31 0 0 1 5.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 0 0-1.134-.175 2.31 2.31 0 0 1-1.64-1.055l-.822-1.316a2.192 2.192 0 0 0-1.736-1.039 48.774 48.774 0 0 0-5.232 0 2.192 2.192 0 0 0-1.736 1.039l-.821 1.316Z" />
                <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 12.75a4.5 4.5 0 1 1-9 0 4.5 4.5 0 0 1 9 0ZM18.75 10.5h.008v.008h-.008V10.5Z" />
              </svg>
              <span className="text-xs">{uploading ? t("uploading") : t("photo.tapToAdd")}</span>
            </span>
          )}
          {uploading && (
            <span className="absolute inset-0 flex items-center justify-center bg-black/60 text-xs text-white">
              {t("uploading")}
            </span>
          )}
        </button>
        <input ref={fileRef} type="file" accept="image/jpeg,image/png,image/webp" capture="user" onChange={handleFileChange} className="hidden" />
        {preview && (
          <button
            type="button"
            onClick={() => fileRef.current?.click()}
            className="text-xs text-gray-400 hover:text-white"
          >
            {t("photo.changePhoto")}
          </button>
        )}
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}
      <UploadRejectionModal rejection={rejection} onClose={clearRejection} />

      <div className="flex flex-col gap-3">
        <Button
          variant="accent"
          className="w-full"
          onClick={handleContinue}
          loading={saving || uploading}
          disabled={saving || uploading}
        >
          {hasNewPhoto || existing ? t("continue") : t("skip")}
        </Button>
      </div>
    </div>
  )
}
