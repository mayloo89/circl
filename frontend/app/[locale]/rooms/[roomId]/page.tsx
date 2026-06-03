"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"
import { useParams } from "next/navigation"

import { Link } from "@/i18n/navigation"
import GuestRoomView from "@/components/chat/GuestRoomView"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function GuestRoomPage() {
  const t = useTranslations("guestRooms")
  const tc = useTranslations("common")
  const params = useParams()
  const roomId = params.roomId as string

  const [sessionId, setSessionId] = useState<string | null>(() => {
    if (typeof window === "undefined") return null
    return sessionStorage.getItem(`guest:session:${roomId}`)
  })
  const [gateNickname, setGateNickname] = useState("")
  const [ageAttestation, setAgeAttestation] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  if (sessionId) {
    return <GuestRoomView roomId={roomId} sessionId={sessionId} />
  }

  async function handleEnter() {
    const nick = gateNickname.trim()
    if (!nick) { setError(t("nicknameRequired")); return }
    if (nick.length > 30) { setError(t("nicknameTooLong")); return }
    if (!ageAttestation) { setError(t("ageAttestRequired")); return }

    setLoading(true)
    setError("")

    try {
      const res = await fetch(`${API_URL}/guest/session`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ nickname: nick, age_attestation: true }),
      })

      if (res.status === 409) {
        setError(t("nicknameTaken"))
        return
      }
      if (!res.ok) {
        setError(tc("unknownError"))
        return
      }

      const data = await res.json() as { session_id: string; nickname: string }
      if (typeof window !== "undefined") {
        sessionStorage.setItem(`guest:session:${roomId}`, data.session_id)
        sessionStorage.setItem(`guest:nickname:${roomId}`, data.nickname)
      }
      setSessionId(data.session_id)
    } catch {
      setError(tc("networkError"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-950 px-4">
      <div className="w-full max-w-sm space-y-6 rounded-xl bg-gray-900 p-6 shadow-2xl ring-1 ring-gray-800">
        <div className="text-center">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-brand-primary/10">
            <svg className="h-7 w-7 text-brand-primary" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5a17.92 17.92 0 01-8.716-2.247m0 0A8.966 8.966 0 013 12c0-1.264.26-2.466.732-3.558" />
            </svg>
          </div>
          <h1 className="mt-4 text-xl font-bold text-foreground">{t("gateTitle")}</h1>
          <p className="mt-1 text-sm text-gray-400">{t("gateSubtitle")}</p>
        </div>

        <div className="space-y-4">
          <Input
            label={t("nicknameLabel")}
            placeholder={t("nicknamePlaceholder")}
            value={gateNickname}
            onChange={(e) => setGateNickname(e.target.value)}
            maxLength={30}
            autoFocus
          />

          <label className="flex cursor-pointer items-start gap-3">
            <input
              type="checkbox"
              checked={ageAttestation}
              onChange={(e) => setAgeAttestation(e.target.checked)}
              className="mt-0.5 h-4 w-4 rounded border-gray-600 bg-gray-800 text-brand-primary focus:ring-brand-hover focus:ring-offset-0"
            />
            <span className="text-sm text-gray-300">{t("ageAttestLabel")}</span>
          </label>

          {error && <p className="text-xs text-red-400">{error}</p>}

          <Button
            variant="accent"
            size="md"
            className="w-full"
            onClick={handleEnter}
            loading={loading}
            disabled={!gateNickname.trim() || !ageAttestation}
          >
            {t("enterRoom")}
          </Button>
        </div>

        <div className="border-t border-gray-800 pt-4 text-center">
          <p className="text-xs text-gray-500">{t("signInPrompt")}</p>
          <Link href="/login" className="mt-1 inline-block text-xs font-medium text-brand-primary hover:text-brand-hover">
            {t("signInCta")}
          </Link>
        </div>
      </div>
    </div>
  )
}
