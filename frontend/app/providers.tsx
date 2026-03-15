"use client"

import { SessionProvider } from "next-auth/react"

import { NotificationsProvider } from "@/contexts/NotificationsContext"
import NavBar from "@/components/NavBar"

export default function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <NotificationsProvider>
        <div className="flex h-screen flex-col overflow-hidden">
          <NavBar />
          <div className="flex-1 overflow-auto min-h-0">
            {children}
          </div>
        </div>
      </NotificationsProvider>
    </SessionProvider>
  )
}
