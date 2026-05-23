"use client"

import { createContext, useCallback, useContext, useEffect, useSyncExternalStore } from "react"

type Theme = "light" | "dark"

interface ThemeContextValue {
  theme: Theme
  toggle: () => void
}

const listeners = new Set<() => void>()

function subscribe(cb: () => void) {
  listeners.add(cb)
  const mq = window.matchMedia("(prefers-color-scheme: dark)")
  mq.addEventListener("change", cb)
  window.addEventListener("storage", cb)
  return () => {
    listeners.delete(cb)
    mq.removeEventListener("change", cb)
    window.removeEventListener("storage", cb)
  }
}

function getSnapshot(): Theme {
  const v = localStorage.getItem("theme")
  if (v === "light" || v === "dark") return v
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
}

const ThemeContext = createContext<ThemeContextValue>({ theme: "dark", toggle: () => {} })

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const theme = useSyncExternalStore(subscribe, getSnapshot, () => "dark" as Theme)

  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark")
  }, [theme])

  const toggle = useCallback(() => {
    const next: Theme = theme === "dark" ? "light" : "dark"
    localStorage.setItem("theme", next)
    document.cookie = `theme=${next}; path=/; max-age=31536000; SameSite=Lax`
    document.documentElement.classList.toggle("dark", next === "dark")
    listeners.forEach((cb) => cb())
  }, [theme])

  return (
    <ThemeContext.Provider value={{ theme, toggle }}>
      {children}
    </ThemeContext.Provider>
  )
}

export const useTheme = () => useContext(ThemeContext)
