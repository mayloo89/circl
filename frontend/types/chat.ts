import type { ChatMessage } from "@/hooks/useChat"

export interface HistoryMessage {
  id: string
  room_id: string
  sender_id: string
  sender_name: string
  sender_avatar_url: string
  type: string
  content: string
  thumbnail_url?: string
  view_once: boolean
  tombstone?: boolean
  redacted?: boolean
  expires_at?: string
  created_at: string
}

export type AnyMessage = HistoryMessage | ChatMessage

export type EphemeralMode = "off" | "view_once" | "15m" | "30m" | "1h" | "6h" | "12h" | "24h"

export const EPHEMERAL_LABELS: Record<EphemeralMode, string> = {
  off: "Off",
  view_once: "View once",
  "15m": "15 minutes",
  "30m": "30 minutes",
  "1h": "1 hour",
  "6h": "6 hours",
  "12h": "12 hours",
  "24h": "24 hours",
}
