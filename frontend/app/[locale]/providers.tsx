"use client"

import { useEffect, useRef } from "react"
import { SessionProvider, signOut, useSession } from "next-auth/react"
import { usePathname, useRouter } from "next/navigation"
import { useLocale } from "next-intl"

import { NotificationsProvider } from "@/contexts/NotificationsContext"
import { PushProvider } from "@/contexts/PushContext"
import { ProfileProvider, useProfileContext } from "@/contexts/ProfileContext"
import { SidebarProvider, useSidebar } from "@/contexts/SidebarContext"
import Sidebar from "@/components/nav/Sidebar"
import TopBar from "@/components/nav/TopBar"
import BottomNav from "@/components/nav/BottomNav"
import PushPrompt from "@/components/PushPrompt"
import { ToastProvider } from "@/components/ui/Toast"

function SessionGuard() {
  const { data: session } = useSession()
  useEffect(() => {
    if (session?.error === "TokenExpired" || session?.error === "RefreshFailed") {
      signOut({ callbackUrl: "/login" })
    }
  }, [session?.error])
  return null
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

function OnboardingRedirect() {
  const { data: session, status } = useSession()
  const { profile, refresh } = useProfileContext()
  const pathname = usePathname()
  const router = useRouter()
  const locale = useLocale()
  const markingRef = useRef(false)

  useEffect(() => {
    if (status !== "authenticated" || profile === null) return
    if (profile.onboarded_at !== null) return
    if (pathname.includes("/onboarding") || pathname.includes("/profile")) return
    if (markingRef.current) return
    markingRef.current = true

    async function markAndRedirect() {
      const token = session?.accessToken
      if (token) {
        try {
          await fetch(`${API_URL}/profiles/me`, {
            method: "PUT",
            headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
            body: JSON.stringify({ mark_onboarded: true }),
          })
          await refresh()
        } catch {}
      }
      router.replace(`/${locale}/onboarding/photo`)
    }
    markAndRedirect()
  }, [status, profile, pathname, router, locale, session, refresh])

  return null
}

function AppShell({ children }: { children: React.ReactNode }) {
  const { status } = useSession()
  const { collapsed } = useSidebar()
  const pathname = usePathname()
  const authenticated = status === "authenticated"
  const isOnboarding = pathname.includes("/onboarding")

  const outerClass = authenticated && !isOnboarding
    ? `flex h-dvh flex-col overflow-hidden pt-14 pb-16 transition-all duration-200 lg:pt-0 lg:pb-0 ${collapsed ? "lg:ml-16" : "lg:ml-64"}`
    : "min-h-dvh"

  return (
    <>
      {authenticated && !isOnboarding && <Sidebar />}
      {authenticated && !isOnboarding && <TopBar />}
      <div className={outerClass}>
        {authenticated && !isOnboarding && <PushPrompt />}
        <div className={authenticated && !isOnboarding ? "flex-1 overflow-auto min-h-0" : "contents"}>
          {children}
        </div>
      </div>
      {authenticated && !isOnboarding && <BottomNav />}
    </>
  )
}

export default function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <SessionGuard />
      <NotificationsProvider>
        <PushProvider>
          <ProfileProvider>
            <SidebarProvider>
              <ToastProvider>
                <OnboardingRedirect />
                <AppShell>{children}</AppShell>
              </ToastProvider>
            </SidebarProvider>
          </ProfileProvider>
        </PushProvider>
      </NotificationsProvider>
    </SessionProvider>
  )
}
