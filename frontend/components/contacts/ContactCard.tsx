"use client"

import { useTranslations } from "next-intl"
import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import PresenceDot from "@/components/ui/PresenceDot"

export type ContactCardVariant = "search-result" | "pending" | "sent" | "contact"

interface ContactCardProps {
  userId: string
  email: string
  displayName: string
  avatarUrl: string
  variant: ContactCardVariant
  online?: boolean
  onNavigate?: () => void
  onPrimary?: () => void
  onSecondary?: () => void
}

export default function ContactCard({
  email,
  displayName,
  avatarUrl,
  variant,
  online,
  onNavigate,
  onPrimary,
  onSecondary,
}: ContactCardProps) {
  const t = useTranslations("contacts")
  const label = displayName || email

  const leftContent =
    variant === "search-result" ? (
      <div className="flex items-center gap-3">
        <Avatar src={avatarUrl} name={label} size="md" />
        <span className="text-sm text-gray-200">{label}</span>
      </div>
    ) : (
      <button onClick={onNavigate} className="flex items-center gap-3 text-left cursor-pointer hover:opacity-80 transition-opacity focus:outline-none focus:ring-2 focus:ring-brand-hover rounded">
        <div className="relative flex-none">
          <Avatar src={avatarUrl} name={label} size="md" />
          {variant === "contact" && (
            <PresenceDot
              online={online ?? false}
              size="md"
              className="absolute -bottom-0.5 -right-0.5 ring-2 ring-gray-900"
            />
          )}
        </div>
        <span className="text-sm text-gray-200">{label}</span>
      </button>
    )

  return (
    <li className="flex items-center justify-between py-2">
      {leftContent}

      <div className="flex gap-2">
        {variant === "search-result" && (
          <Button variant="primary" size="sm" onClick={onPrimary}>{t("add")}</Button>
        )}
        {variant === "pending" && (
          <>
            <Button variant="accent" size="sm" onClick={onPrimary}>{t("accept")}</Button>
            <Button variant="secondary" size="sm" onClick={onSecondary}>{t("decline")}</Button>
          </>
        )}
        {variant === "sent" && (
          <Button variant="secondary" size="sm" onClick={onSecondary}>{t("cancel")}</Button>
        )}
        {variant === "contact" && (
          <>
            <Button variant="accent" size="sm" onClick={onPrimary}>{t("message")}</Button>
            <Button variant="danger" size="sm" onClick={onSecondary}>{t("remove")}</Button>
          </>
        )}
      </div>
    </li>
  )
}
