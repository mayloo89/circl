"use client"

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react"
import { useSession } from "next-auth/react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export interface MyProfile {
  username: string
  display_name: string
  avatar_url: string
  onboarded_at: string | null
}

interface ProfileContextValue {
  profile: MyProfile | null
  refresh: () => void
}

const ProfileContext = createContext<ProfileContextValue>({
  profile: null,
  refresh: () => {},
})

export function ProfileProvider({ children }: { children: React.ReactNode }) {
  const { data: session, status } = useSession()
  const [profile, setProfile] = useState<MyProfile | null>(null)
  const tokenRef = useRef<string | undefined>(undefined)

  useEffect(() => {
    tokenRef.current = session?.accessToken
  })

  async function doFetch(token: string) {
    try {
      const res = await fetch(`${API_URL}/profiles/me`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.ok) {
        const data = await res.json()
        setProfile({
          username: data.username ?? "",
          display_name: data.display_name ?? "",
          avatar_url: data.avatar_url ?? "",
          onboarded_at: data.onboarded_at ?? null,
        })
      }
    } catch {
      // non-critical
    }
  }

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) return
    const token = session.accessToken
    let cancelled = false
    fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => {
        if (!cancelled) {
          setProfile({
            username: data.username ?? "",
            display_name: data.display_name ?? "",
            avatar_url: data.avatar_url ?? "",
            onboarded_at: data.onboarded_at ?? null,
          })
        }
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [status, session?.accessToken])

  const refresh = useCallback(() => {
    const token = tokenRef.current
    if (token) doFetch(token)
  }, [])

  return (
    <ProfileContext.Provider value={{ profile, refresh }}>
      {children}
    </ProfileContext.Provider>
  )
}

export function useProfileContext() {
  return useContext(ProfileContext)
}
