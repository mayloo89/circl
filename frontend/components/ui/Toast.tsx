"use client"

import { createContext, useCallback, useContext, useState } from "react"

// ─── Types ─────────────────────────────────────────────────────────────────────

type ToastType = "success" | "error" | "info"

interface ToastItem {
  id: string
  message: string
  type: ToastType
}

// ─── Context ───────────────────────────────────────────────────────────────────

interface ToastContextValue {
  toast: (message: string, type?: ToastType) => void
}

const ToastContext = createContext<ToastContextValue>({ toast: () => {} })

let seq = 0

// ─── Provider ──────────────────────────────────────────────────────────────────

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const toast = useCallback((message: string, type: ToastType = "info") => {
    const id = String(seq++)
    setToasts((prev) => [...prev, { id, message, type }])
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 4_000)
  }, [])

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      {toasts.length > 0 && (
        <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2" role="status" aria-live="polite">
          {toasts.map((t) => (
            <div
              key={t.id}
              className={`rounded-lg px-4 py-3 text-sm text-white shadow-lg ring-1 ${
                t.type === "success"
                  ? "bg-green-800 ring-green-700"
                  : t.type === "error"
                  ? "bg-red-900 ring-red-800"
                  : "bg-gray-800 ring-gray-700"
              }`}
            >
              {t.message}
            </div>
          ))}
        </div>
      )}
    </ToastContext.Provider>
  )
}

// ─── Hook ──────────────────────────────────────────────────────────────────────

export function useToast(): ToastContextValue {
  return useContext(ToastContext)
}
