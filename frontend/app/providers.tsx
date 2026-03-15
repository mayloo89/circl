"use client"

import { SessionProvider } from "next-auth/react"

import { NotificationsProvider } from "@/contexts/NotificationsContext"
import NavBar from "@/components/NavBar"

export default function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <NotificationsProvider>
        <NavBar />
        {children}
      </NotificationsProvider>
    </SessionProvider>
  )
}
