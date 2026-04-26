"use client"

import { useEffect } from "react"
import { SessionProvider, signOut, useSession } from "next-auth/react"

import { NotificationsProvider } from "@/contexts/NotificationsContext"
import { PushProvider } from "@/contexts/PushContext"
import { ProfileProvider } from "@/contexts/ProfileContext"
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

export default function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <SessionGuard />
      <NotificationsProvider>
        <PushProvider>
          <ProfileProvider>
            <ToastProvider>
              {/* Desktop sidebar (lg+) */}
              <Sidebar />
              {/* Mobile top bar */}
              <TopBar />
              {/* Main content — clears fixed bars */}
              <div className="min-h-dvh pt-14 pb-16 lg:ml-64 lg:pt-0 lg:pb-0">
                <PushPrompt />
                {children}
              </div>
              {/* Mobile bottom nav */}
              <BottomNav />
            </ToastProvider>
          </ProfileProvider>
        </PushProvider>
      </NotificationsProvider>
    </SessionProvider>
  )
}
