"use client"

import { useTranslations } from "next-intl"

import ChatListPane from "@/components/chat/ChatListPane"

export default function ChatPage() {
  const t = useTranslations("chat")

  return (
    <div className="flex h-full bg-gray-950">
      {/* Mobile (<lg): list takes the full viewport. */}
      <div className="flex h-full w-full lg:hidden">
        <ChatListPane variant="page" />
      </div>

      {/* Desktop (lg+): split — list on the left, empty-state pane on the right. */}
      <div className="hidden h-full w-full lg:flex">
        <div className="flex h-full w-96 shrink-0 flex-col border-r border-gray-800">
          <ChatListPane variant="pane" />
        </div>
        <div className="flex flex-1 items-center justify-center bg-gray-950 px-8 text-center">
          <div className="max-w-sm space-y-3">
            <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-gray-800">
              <svg className="h-8 w-8 text-gray-600" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
              </svg>
            </div>
            <p className="text-sm font-medium text-gray-300">{t("selectConversationTitle")}</p>
            <p className="text-xs text-gray-500">{t("selectConversationDesc")}</p>
          </div>
        </div>
      </div>
    </div>
  )
}
