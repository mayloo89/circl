"use client"

import { useEffect } from "react"
import { SessionProvider, signOut, useSession } from "next-auth/react"

import { NotificationsProvider } from "@/contexts/NotificationsContext"
import { PushProvider } from "@/contexts/PushContext"
import NavBar from "@/components/NavBar"
import PushPrompt from "@/components/PushPrompt"
import { ToastProvider } from "@/components/ui/Toast"

// Signs out automatically when the backend JWT has expired so the user is
// redirected to login instead of seeing a broken app full of 401 errors.
function SessionGuard() {
  const { data: session } = useSession()
  useEffect(() => {
    if (session?.error === "TokenExpired") {
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
        <ToastProvider>
          <div className="flex h-screen flex-col overflow-hidden">
            <NavBar />
            <PushPrompt />
            <div className="flex-1 overflow-auto min-h-0">
              {children}
            </div>
          </div>
        </ToastProvider>
        </PushProvider>
      </NotificationsProvider>
    </SessionProvider>
  )
}
