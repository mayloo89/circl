"use client"

import { useEffect, useRef } from "react"
import { SessionProvider, signOut, useSession } from "next-auth/react"
import { usePathname, useRouter } from "next/navigation"
import { useLocale, useTranslations } from "next-intl"

import { firstIncompleteStep } from "@/lib/onboardingSteps"
import { NotificationsProvider } from "@/contexts/NotificationsContext"
import { PushProvider } from "@/contexts/PushContext"
import { ProfileProvider, useProfileContext } from "@/contexts/ProfileContext"
import { SidebarProvider, useSidebar } from "@/contexts/SidebarContext"
import Sidebar from "@/components/nav/Sidebar"
import TopBar from "@/components/nav/TopBar"
import BottomNav from "@/components/nav/BottomNav"
import PushPrompt from "@/components/PushPrompt"
import AuthLocalePicker from "@/components/AuthLocalePicker"
import { ToastProvider } from "@/components/ui/Toast"

function SessionGuard() {
  const { data: session } = useSession()
  const signingOut = useRef(false)
  useEffect(() => {
    if (signingOut.current) return
    if (session?.error === "TokenExpired" || session?.error === "RefreshFailed") {
      signingOut.current = true
      signOut({ callbackUrl: "/login", redirect: true })
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

    const first = firstIncompleteStep(profile)
    if (!first) {
      // All fields already filled — mark onboarded silently and stay on home
      const token = session?.accessToken
      if (token) {
        fetch(`${API_URL}/profiles/me`, {
          method: "PUT",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({ mark_onboarded: true }),
        })
          .then(() => refresh())
          .catch(() => {})
      }
    } else {
      router.replace(`/${locale}/onboarding/${first}`)
    }
  }, [status, profile, pathname, router, locale, session, refresh])

  return null
}

function AppShell({ children }: { children: React.ReactNode }) {
  const { status } = useSession()
  const { collapsed } = useSidebar()
  const pathname = usePathname()
  const tNav = useTranslations("nav")
  const authenticated = status === "authenticated"
  const unauthenticated = status === "unauthenticated"
  const isOnboarding = pathname.includes("/onboarding")

  const outerClass = authenticated && !isOnboarding
    ? `flex h-dvh flex-col overflow-hidden pt-topbar pb-bottomnav transition-all duration-200 lg:pt-0 lg:pb-0 ${collapsed ? "lg:ml-16" : "lg:ml-64"}`
    : "min-h-dvh"

  return (
    <>
      {unauthenticated && <AuthLocalePicker />}
      {authenticated && !isOnboarding && (
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[100] focus:rounded-md focus:bg-brand-primary focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-white focus:shadow-lg focus:outline-none focus:ring-2 focus:ring-white"
        >
          {tNav("skipToContent")}
        </a>
      )}
      {authenticated && !isOnboarding && <Sidebar />}
      {authenticated && !isOnboarding && <TopBar />}
      <div className={outerClass}>
        {authenticated && !isOnboarding && <PushPrompt />}
        {authenticated && !isOnboarding ? (
          <main id="main-content" className="flex-1 overflow-auto min-h-0">
            {children}
          </main>
        ) : (
          <div className="contents">{children}</div>
        )}
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
