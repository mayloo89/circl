"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslations } from "next-intl"

import type { AnyMessage } from "@/types/chat"
import type { ParticipantEvent } from "@/hooks/useChat"
import { useMenuKeyboard } from "@/hooks/useMenuKeyboard"
import { useToast } from "@/components/ui/Toast"
import MessageBubble from "@/components/chat/MessageBubble"
import ChatInput from "@/components/chat/ChatInput"
import DateSeparator from "@/components/chat/DateSeparator"
import TypingIndicator from "@/components/chat/TypingIndicator"
import Avatar from "@/components/ui/Avatar"
import ConfirmDialog from "@/components/ui/ConfirmDialog"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface Participant {
  userId: string
  displayName: string
  avatarURL: string
  isGuest: boolean
}

interface RosterRow {
  user_id: string
  display_name: string
  avatar_url: string
  is_guest?: boolean
}

interface PublicRoomShellProps {
  roomId: string
  roomName: string
  /** When set, the roster is seeded from the authenticated members endpoint. */
  token?: string
  messages: AnyMessage[]
  historyLoaded: boolean
  connected: boolean
  deletedIds: Set<string>
  typingNames: string[]
  participantEvents: ParticipantEvent[]
  /** True when the given sender id is the current viewer (own message). */
  isOwn: (senderId: string) => boolean
  /** Optional pill rendered in the header (e.g. the viewer's guest tag). */
  headerBadge?: string
  /** When true, leaving the room asks for confirmation first (guests lose
   *  their ephemeral nickname/session on leave). */
  confirmOnLeave?: boolean
  /** Whether the current viewer was removed from the room by a moderator. */
  isKicked?: boolean
  /** Whether the current viewer is currently muted in this room. */
  isMuted?: boolean
  /** Whether the current viewer is a platform admin (shows mod actions). */
  isAdmin?: boolean
  /** The viewer's own user ID — used to suppress mod actions on self. */
  viewerId?: string
  onBack: () => void
  onSend: (content: string) => void
  onTyping: () => void
}

/**
 * Presentational shell shared by the guest and registered public-room views.
 * Public rooms are text-only on both paths, so attachment and ephemeral
 * controls are disabled here regardless of who is viewing.
 *
 * The participant rail mirrors the channel members rail (right pane + filter)
 * but shows display name + badge only — never a registered user's @handle.
 * The roster is seeded once from the room's live participants (the
 * authenticated members endpoint for registered users, the guest-safe endpoint
 * otherwise) and kept current from join/leave events.
 */
export default function PublicRoomShell({
  roomId,
  roomName,
  token,
  messages,
  historyLoaded,
  connected,
  deletedIds,
  typingNames,
  participantEvents,
  isOwn,
  headerBadge,
  confirmOnLeave = false,
  isKicked = false,
  isMuted = false,
  isAdmin = false,
  viewerId,
  onBack,
  onSend,
  onTyping,
}: PublicRoomShellProps) {
  const t = useTranslations("guestRooms")
  const tr = useTranslations("chatRoom")
  const [now] = useState(() => Date.now())
  const [seed, setSeed] = useState<Participant[]>([])
  const [rosterOpen, setRosterOpen] = useState(true)
  const [memberQuery, setMemberQuery] = useState("")
  const [leaveConfirmOpen, setLeaveConfirmOpen] = useState(false)
  const [openModMenuId, setOpenModMenuId] = useState<string | null>(null)
  const [menuPos, setMenuPos] = useState<{ top: number; right: number } | null>(null)
  const modMenuRef = useRef<HTMLDivElement | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const { toast } = useToast()

  const closeModMenu = useCallback(() => { setOpenModMenuId(null); setMenuPos(null) }, [])

  useMenuKeyboard({ open: openModMenuId !== null, containerRef: modMenuRef, onClose: closeModMenu })

  useEffect(() => {
    if (openModMenuId === null) return
    function handleOutside(e: MouseEvent) {
      if (modMenuRef.current && !modMenuRef.current.contains(e.target as Node)) closeModMenu()
    }
    document.addEventListener("mousedown", handleOutside)
    return () => document.removeEventListener("mousedown", handleOutside)
  }, [openModMenuId, closeModMenu])

  function openModMenu(userId: string, trigger: HTMLButtonElement) {
    if (openModMenuId === userId) {
      setOpenModMenuId(null)
      setMenuPos(null)
      return
    }
    const rect = trigger.getBoundingClientRect()
    setMenuPos({ top: rect.bottom + 4, right: window.innerWidth - rect.right })
    setOpenModMenuId(userId)
  }

  const modAction = useCallback(async (path: string, body: Record<string, string>, successKey: "kickDone" | "muteDone") => {
    if (!token) return
    setOpenModMenuId(null)
    setMenuPos(null)
    const res = await fetch(`${API_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify(body),
    }).catch(() => null)
    if (res?.ok) toast(tr(successKey), "success")
  }, [token, toast, tr])

  function kick(targetId: string) {
    void modAction(`/chat/rooms/${roomId}/mod/kick`, { target_id: targetId }, "kickDone")
  }

  function mute(targetId: string, duration: string) {
    void modAction(`/chat/rooms/${roomId}/mod/mute`, { target_id: targetId, duration }, "muteDone")
  }

  function handleBack() {
    if (confirmOnLeave) setLeaveConfirmOpen(true)
    else onBack()
  }

  useEffect(() => {
    let active = true
    const url = token
      ? `${API_URL}/chat/rooms/${roomId}/members`
      : `${API_URL}/guest/rooms/${roomId}/participants`
    fetch(url, token ? { headers: { Authorization: `Bearer ${token}` } } : undefined)
      .then((r) => (r.ok ? r.json() : []))
      .then((list: RosterRow[]) => {
        if (!active) return
        setSeed((Array.isArray(list) ? list : []).map((p) => ({
          userId: p.user_id,
          displayName: p.display_name,
          avatarURL: p.avatar_url,
          isGuest: Boolean(p.is_guest),
        })))
      })
      .catch(() => {})
    return () => { active = false }
  }, [roomId, token])

  // Roster = the seed snapshot with every join/leave event applied on top.
  const roster = useMemo(() => {
    const m = new Map<string, Participant>()
    for (const p of seed) m.set(p.userId, p)
    for (const ev of participantEvents) {
      if (ev.type === "join") {
        m.set(ev.userId, { userId: ev.userId, displayName: ev.displayName, avatarURL: ev.avatarURL, isGuest: ev.isGuest })
      } else {
        m.delete(ev.userId)
      }
    }
    return [...m.values()]
  }, [seed, participantEvents])

  const visibleRoster = useMemo(() => {
    const q = memberQuery.toLowerCase()
    return roster
      .filter((p) => !q || (p.displayName || "").toLowerCase().startsWith(q))
      .sort((a, b) => (a.displayName || "").localeCompare(b.displayName || ""))
  }, [roster, memberQuery])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [messages.length])

  function formatDay(dateStr: string) {
    return new Date(dateStr).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })
  }

  return (
    <div className="relative flex h-screen bg-gray-950">
      <ConfirmDialog
        open={leaveConfirmOpen}
        title={t("leaveTitle")}
        message={t("leaveMessage")}
        confirmLabel={tr("leave")}
        onConfirm={() => { setLeaveConfirmOpen(false); onBack() }}
        onCancel={() => setLeaveConfirmOpen(false)}
      />

      {isKicked && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-gray-950/90 backdrop-blur-sm">
          <div className="mx-4 w-full max-w-sm rounded-xl bg-gray-900 p-6 text-center shadow-2xl ring-1 ring-gray-800">
            <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-500/10">
              <svg className="h-6 w-6 text-red-400" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
              </svg>
            </div>
            <h2 className="text-base font-semibold text-foreground">{t("kickedTitle")}</h2>
            <p className="mt-1 text-sm text-gray-400">{t("youWereKicked")}</p>
            <button
              type="button"
              onClick={onBack}
              className="mt-4 w-full cursor-pointer rounded-lg bg-brand-primary px-4 py-2 text-sm font-medium text-white hover:bg-brand-hover focus:outline-none focus:ring-2 focus:ring-brand-hover"
            >
              {t("backToRooms")}
            </button>
          </div>
        </div>
      )}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center gap-3 border-b border-gray-800 bg-gray-900 px-4 py-3">
          <button
            type="button"
            onClick={handleBack}
            className="cursor-pointer rounded-full p-1.5 text-gray-400 hover:bg-gray-800 hover:text-gray-200 focus:outline-none focus:ring-2 focus:ring-brand-hover"
            aria-label={t("backToRooms")}
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
            </svg>
          </button>
          <Avatar name={roomName || roomId} size="sm" color="indigo" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold text-foreground">{roomName || roomId}</p>
          </div>
          {headerBadge && (
            <span className="rounded-full bg-brand-primary/10 px-2.5 py-0.5 text-xs font-medium text-brand-primary">
              {headerBadge}
            </span>
          )}
          <button
            type="button"
            onClick={() => setRosterOpen((v) => !v)}
            className={`flex shrink-0 cursor-pointer items-center gap-1.5 rounded p-1.5 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-brand-hover ${rosterOpen ? "bg-gray-700 text-white" : "text-gray-400 hover:bg-gray-800 hover:text-white"}`}
            aria-label={rosterOpen ? tr("hideMembers") : tr("showMembers")}
            aria-expanded={rosterOpen}
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
            </svg>
            <span className="text-xs font-medium">{roster.length}</span>
          </button>
        </header>

        {!historyLoaded ? (
          <div className="flex flex-1 items-center justify-center">
            <p className="text-sm text-gray-500">{tr("loading")}</p>
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto px-4 py-4">
            {messages.map((msg, i) => {
              const msgDate = formatDay(msg.created_at)
              const showDateSep = i === 0 || formatDay(messages[i - 1].created_at) !== msgDate
              const firstInGroup = i === 0 || messages[i - 1].sender_id !== msg.sender_id
              const lastInGroup = i === messages.length - 1 || messages[i + 1]?.sender_id !== msg.sender_id

              return (
                <div key={msg.id}>
                  {showDateSep && <DateSeparator label={msgDate} />}
                  <MessageBubble
                    msg={msg}
                    isOwn={isOwn(msg.sender_id)}
                    firstInGroup={firstInGroup}
                    lastInGroup={lastInGroup}
                    isLive={false}
                    isTombstone={deletedIds.has(msg.id)}
                    revealedText={undefined}
                    isLastSeenOwn={false}
                    now={now}
                    onViewOnce={() => {}}
                    onOpenMedia={() => {}}
                  />
                </div>
              )
            })}
            <div ref={bottomRef} />
          </div>
        )}

        <TypingIndicator typers={typingNames} />

        {isMuted && (
          <div className="flex items-center gap-2 border-t border-amber-500/20 bg-amber-500/10 px-4 py-2" role="status">
            <svg className="h-4 w-4 shrink-0 text-amber-400" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M17.25 9.75L19.5 12m0 0l2.25 2.25M19.5 12l2.25-2.25M19.5 12l-2.25 2.25m-10.5-6l4.72-4.72a.75.75 0 011.28.53v15.88a.75.75 0 01-1.28.53l-4.72-4.72H4.51c-.88 0-1.704-.507-1.938-1.354A9.01 9.01 0 012.25 12c0-.83.112-1.633.322-2.396C2.806 8.756 3.63 8.25 4.51 8.25H6.75z" />
            </svg>
            <span className="text-xs text-amber-300">{t("youAreMuted")}</span>
          </div>
        )}

        <ChatInput
          connected={connected}
          uploading={false}
          ephemeral="off"
          onEphemeralChange={() => {}}
          onSend={onSend}
          onAttach={() => {}}
          onTyping={onTyping}
          disableAttach={true}
          disableEphemeral={true}
        />
      </div>

      {rosterOpen && (
        <aside className="hidden w-52 shrink-0 flex-col border-l border-gray-800 bg-gray-900 sm:flex">
          <div className="space-y-2 px-3 pb-2 pt-3">
            <p className="text-[11px] font-semibold uppercase tracking-wider text-gray-500">
              {tr("membersTitle", { count: roster.length })}
            </p>
            <input
              type="text"
              value={memberQuery}
              onChange={(e) => setMemberQuery(e.target.value)}
              placeholder={tr("filterMembers")}
              className="w-full rounded bg-gray-800 px-2 py-1 text-xs text-gray-200 placeholder-gray-600 outline-none focus:ring-1 focus:ring-brand-hover"
            />
          </div>
          {visibleRoster.length === 0 ? (
            <p className="px-3 py-2 text-xs text-gray-500">{t("noOneHere")}</p>
          ) : (
            <ul className="overflow-y-auto">
              {visibleRoster.map((p) => (
                <li key={p.userId} className="flex items-center gap-2 px-3 py-2 hover:bg-gray-800/50">
                  <div className="relative shrink-0">
                    <Avatar src={p.avatarURL || undefined} name={p.displayName || "?"} size="xs" color="indigo" />
                    <span className="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-green-400 ring-2 ring-gray-900" aria-hidden="true" />
                  </div>
                  <span className="min-w-0 flex-1 truncate text-xs font-medium text-foreground">{p.displayName || "—"}</span>
                  {p.isGuest && (
                    <span className="shrink-0 rounded bg-gray-800 px-1.5 py-0.5 text-[10px] font-medium text-gray-400">
                      {t("guestBadge")}
                    </span>
                  )}
                  {isAdmin && p.userId !== viewerId && (
                    <button
                      type="button"
                      onClick={(e) => openModMenu(p.userId, e.currentTarget)}
                      aria-haspopup="menu"
                      aria-expanded={openModMenuId === p.userId}
                      aria-label={tr("moderationMenu")}
                      className="shrink-0 cursor-pointer rounded p-0.5 text-gray-500 hover:bg-gray-700 hover:text-gray-200 focus:outline-none focus:ring-1 focus:ring-brand-hover"
                    >
                      <svg className="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                        <path d="M10 3a1.5 1.5 0 110 3 1.5 1.5 0 010-3zM10 8.5a1.5 1.5 0 110 3 1.5 1.5 0 010-3zM11.5 15.5a1.5 1.5 0 10-3 0 1.5 1.5 0 003 0z" />
                      </svg>
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </aside>
      )}

      {openModMenuId !== null && menuPos !== null && (
        <div
          ref={modMenuRef}
          role="menu"
          style={{ position: "fixed", top: menuPos.top, right: menuPos.right }}
          className="z-50 w-40 overflow-hidden rounded-lg bg-gray-800 py-1 shadow-xl ring-1 ring-gray-700 focus:outline-none"
        >
          <button
            role="menuitem"
            type="button"
            onClick={() => kick(openModMenuId)}
            className="flex w-full cursor-pointer items-center px-3 py-2 text-xs text-red-400 hover:bg-gray-700 focus:bg-gray-700 focus:outline-none"
          >
            {tr("kickAction")}
          </button>
          <button
            role="menuitem"
            type="button"
            onClick={() => mute(openModMenuId, "15m")}
            className="flex w-full cursor-pointer items-center px-3 py-2 text-xs text-gray-200 hover:bg-gray-700 focus:bg-gray-700 focus:outline-none"
          >
            {tr("mute15m")}
          </button>
          <button
            role="menuitem"
            type="button"
            onClick={() => mute(openModMenuId, "1h")}
            className="flex w-full cursor-pointer items-center px-3 py-2 text-xs text-gray-200 hover:bg-gray-700 focus:bg-gray-700 focus:outline-none"
          >
            {tr("mute1h")}
          </button>
          <button
            role="menuitem"
            type="button"
            onClick={() => mute(openModMenuId, "24h")}
            className="flex w-full cursor-pointer items-center px-3 py-2 text-xs text-gray-200 hover:bg-gray-700 focus:bg-gray-700 focus:outline-none"
          >
            {tr("mute24h")}
          </button>
        </div>
      )}
    </div>
  )
}
