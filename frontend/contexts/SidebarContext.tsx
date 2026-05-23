"use client"

import { createContext, useCallback, useContext, useSyncExternalStore } from "react"

interface SidebarContextValue {
  collapsed: boolean
  toggle: () => void
}

const SidebarContext = createContext<SidebarContextValue>({
  collapsed: false,
  toggle: () => {},
})

const sidebarListeners = new Set<() => void>()

function subscribe(cb: () => void) {
  sidebarListeners.add(cb)
  return () => { sidebarListeners.delete(cb) }
}

function getSnapshot(): boolean {
  return localStorage.getItem("sidebar-collapsed") === "true"
}

export function SidebarProvider({ children }: { children: React.ReactNode }) {
  const collapsed = useSyncExternalStore(subscribe, getSnapshot, () => false)

  const toggle = useCallback(() => {
    const next = !collapsed
    localStorage.setItem("sidebar-collapsed", String(next))
    sidebarListeners.forEach((cb) => cb())
  }, [collapsed])

  return (
    <SidebarContext.Provider value={{ collapsed, toggle }}>
      {children}
    </SidebarContext.Provider>
  )
}

export function useSidebar() {
  return useContext(SidebarContext)
}
