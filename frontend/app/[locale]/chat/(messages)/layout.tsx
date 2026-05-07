"use client"

import { useParams } from "next/navigation"
import { useTranslations } from "next-intl"

import ChatListPane from "@/components/chat/ChatListPane"

/**
 * Layout shared by /chat (DM list index) and /chat/[roomId] (DM/group room).
 *
 * The DM list lives here, not in the page, so it persists across navigation
 * between /chat and /chat/<id> and between threads — no flicker, no refetch,
 * no scroll-position loss when switching conversations.
 *
 * On desktop (`lg:`) the list is rendered as a fixed-width side rail; the
 * page renders into the right column (`children`). On mobile the side rail
 * is hidden and the page owns the full viewport (the index page renders the
 * list at full width, the room page renders the conversation at full width).
 *
 * Channels live OUTSIDE this layout (under /chat/channels/...) so they never
 * inherit the DM list pane — accidental channel-switching could erase
 * membership and message access, so channels stay deliberately single-pane.
 */
export default function MessagesLayout({ children }: { children: React.ReactNode }) {
  const t = useTranslations("chat")
  const params = useParams()
  const selectedRoomId = typeof params.roomId === "string" ? params.roomId : undefined

  return (
    <div className="flex h-full bg-gray-950">
      <aside
        aria-label={t("conversationsList")}
        className="hidden lg:flex lg:h-full lg:w-96 lg:shrink-0 lg:flex-col lg:border-r lg:border-gray-800"
      >
        <ChatListPane selectedRoomId={selectedRoomId} variant="pane" />
      </aside>
      <div className="flex flex-1 flex-col overflow-hidden">{children}</div>
    </div>
  )
}
