"use client"

import { useEffect } from "react"
import { SessionProvider, signOut, useSession } from "next-auth/react"

import { NotificationsProvider } from "@/contexts/NotificationsContext"
import { PushProvider } from "@/contexts/PushContext"
import { ProfileProvider } from "@/contexts/ProfileContext"
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

function AppShell({ children }: { children: React.ReactNode }) {
  const { status } = useSession()
  const { collapsed } = useSidebar()
  const authenticated = status === "authenticated"

  const outerClass = authenticated
    ? `flex h-dvh flex-col overflow-hidden pt-14 pb-16 transition-all duration-200 lg:pt-0 lg:pb-0 ${collapsed ? "lg:ml-16" : "lg:ml-64"}`
    : "min-h-dvh"

  return (
    <>
      {authenticated && <Sidebar />}
      {authenticated && <TopBar />}
      <div className={outerClass}>
        {authenticated && <PushPrompt />}
        <div className={authenticated ? "flex-1 overflow-auto min-h-0" : "contents"}>
          {children}
        </div>
      </div>
      {authenticated && <BottomNav />}
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
                <AppShell>{children}</AppShell>
              </ToastProvider>
            </SidebarProvider>
          </ProfileProvider>
        </PushProvider>
      </NotificationsProvider>
    </SessionProvider>
  )
}
