"use client"

import { useTranslations } from "next-intl"

import Button from "@/components/ui/Button"
import Modal from "@/components/ui/Modal"
import { Link } from "@/i18n/navigation"

export type UploadRejectionCode =
  | "nsfw_detected"
  | "hash_match"
  | "size_out_of_bounds"
  | "aspect_ratio_out_of_bounds"

export interface UploadRejection {
  code: UploadRejectionCode | string
  reason?: string
}

interface Props {
  rejection: UploadRejection | null
  onClose: () => void
  // pendingReview shows a non-error "still being reviewed" variant when the
  // moderation poll timed out while the server still held the image (e.g. a
  // vendor outage). Ignored when a rejection is present (rejection wins).
  pendingReview?: boolean
}

const KNOWN_CODES: ReadonlySet<string> = new Set([
  "nsfw_detected",
  "hash_match",
  "size_out_of_bounds",
  "aspect_ratio_out_of_bounds",
])

/**
 * UploadRejectionModal is shown after the moderation pipeline blocks an
 * upload. The body text is localized off the machine-readable code (one
 * string per locale per category), with the backend's free-form `reason`
 * shown as fine print below — useful for admins and developers debugging
 * an unexpected rejection without forcing the end-user to parse it.
 */
export default function UploadRejectionModal({ rejection, onClose, pendingReview }: Props) {
  const t = useTranslations("uploadRejection")

  // Pending variant: not an error — the image is still under review. Rejection
  // takes precedence when both are set.
  if (!rejection && pendingReview) {
    return (
      <Modal open onClose={onClose}>
        <div
          className="w-full max-w-md rounded-xl bg-gray-900 p-6 shadow-2xl ring-1 ring-gray-700"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-start gap-3">
            <span
              aria-hidden="true"
              className="mt-0.5 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-500/15 text-amber-400"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth={2}
                strokeLinecap="round"
                strokeLinejoin="round"
                className="h-5 w-5"
              >
                <circle cx="12" cy="12" r="10" />
                <path d="M12 6v6l4 2" />
              </svg>
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="text-lg font-semibold text-foreground">{t("pendingTitle")}</h2>
              <p className="mt-1 text-sm text-gray-300">{t("pendingBody")}</p>
            </div>
          </div>
          <div className="mt-5 flex items-center justify-end">
            <Button variant="primary" size="sm" onClick={onClose}>
              {t("dismiss")}
            </Button>
          </div>
        </div>
      </Modal>
    )
  }

  if (!rejection) return null

  const codeKey = KNOWN_CODES.has(rejection.code) ? rejection.code : "unknown"
  const body = t(`code_${codeKey}`)

  return (
    <Modal open onClose={onClose}>
      <div
        className="w-full max-w-md rounded-xl bg-gray-900 p-6 shadow-2xl ring-1 ring-gray-700"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start gap-3">
          <span
            aria-hidden="true"
            className="mt-0.5 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-rose-500/15 text-rose-400"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth={2}
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-5 w-5"
            >
              <path d="M12 9v4" />
              <path d="M12 17h.01" />
              <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
            </svg>
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-lg font-semibold text-foreground">{t("title")}</h2>
            <p className="mt-1 text-sm text-gray-300">{t("intro")}</p>
            <p className="mt-3 text-sm text-gray-200">{body}</p>
            <p className="mt-2 text-xs text-gray-500">{t("fallback")}</p>
            {rejection.reason ? (
              <p className="mt-3 text-[11px] uppercase tracking-wide text-gray-600">
                {t("reasonLabel")}:{" "}
                <span className="font-mono text-gray-500 normal-case">{rejection.reason}</span>
              </p>
            ) : null}
          </div>
        </div>

        <div className="mt-5 flex items-center justify-between gap-3">
          <Link
            href="/guidelines"
            className="text-xs text-gray-400 underline-offset-4 hover:text-foreground hover:underline"
          >
            {t("guidelinesLink")}
          </Link>
          <Button variant="primary" size="sm" onClick={onClose}>
            {t("dismiss")}
          </Button>
        </div>
      </div>
    </Modal>
  )
}
