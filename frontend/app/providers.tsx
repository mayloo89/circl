"use client"

import { SessionProvider } from "next-auth/react"

import { NotificationsProvider } from "@/contexts/NotificationsContext"
import NavBar from "@/components/NavBar"
import { ToastProvider } from "@/components/ui/Toast"

export default function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <NotificationsProvider>
        <ToastProvider>
          <div className="flex h-screen flex-col overflow-hidden">
            <NavBar />
            <div className="flex-1 overflow-auto min-h-0">
              {children}
            </div>
          </div>
        </ToastProvider>
      </NotificationsProvider>
    </SessionProvider>
  )
}
