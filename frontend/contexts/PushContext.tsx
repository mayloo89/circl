"use client"

import { createContext, useContext } from "react"
import { useSession } from "next-auth/react"

import { usePush, type PushPermission } from "@/hooks/usePush"

interface PushContextValue {
  permission: PushPermission
  supported: boolean
  enable: () => Promise<void>
  disable: () => Promise<void>
}

const PushContext = createContext<PushContextValue | null>(null)

export function PushProvider({ children }: { children: React.ReactNode }) {
  const { data: session } = useSession()
  const push = usePush(session?.accessToken)

  return <PushContext.Provider value={push}>{children}</PushContext.Provider>
}

export function usePushContext(): PushContextValue {
  const ctx = useContext(PushContext)
  if (!ctx) throw new Error("usePushContext must be used inside PushProvider")
  return ctx
}
