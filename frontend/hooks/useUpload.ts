"use client"

import { useState } from "react"

import type { UploadRejection } from "@/components/upload/UploadRejectionModal"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export interface UploadResult {
  upload_id: string
  storage_key: string
  url: string
}

type UploadCategory = "avatar" | "chat-attachment" | "gallery" | "album-private"

const MAX_SIZES: Record<UploadCategory, number> = {
  avatar: 5 * 1024 * 1024,
  "chat-attachment": 50 * 1024 * 1024,
  gallery: 10 * 1024 * 1024,
  "album-private": 15 * 1024 * 1024,
}

const ALLOWED_TYPES: Record<UploadCategory, string[]> = {
  avatar: ["image/jpeg", "image/png", "image/webp"],
  "chat-attachment": ["image/jpeg", "image/png", "image/webp", "image/gif", "video/mp4", "video/quicktime", "application/pdf"],
  gallery: ["image/jpeg", "image/png", "image/webp"],
  "album-private": ["image/jpeg", "image/png", "image/webp"],
}

interface ModerationVerdict {
  status: "approved" | "rejected" | "pending"
  code?: string
  reason?: string
}

// Normal moderation completes in ~1s. The backend holds-and-retries on a
// vendor outage (legal-floor detectors fail closed), so a `pending` row can
// persist far longer; we poll for a bounded window and then report `pending`
// rather than ever faking approval — sending an un-approved image would be
// rejected by the server's attach-time gate anyway.
const POLL_TIMEOUT_MS = 12000
const POLL_INTERVAL_MS = 500

// pollModeration GETs the upload row until moderation completes or the poll
// window elapses. A poll-call failure (network blip) is retried on the next
// tick; only a definitive `rejected`/`quarantined` row is treated as a content
// block. Timing out returns `pending` — the image is still under review, not
// approved, so the caller must not send it.
async function pollModeration(uploadID: string, token: string, timeoutMs: number): Promise<ModerationVerdict> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    // Bound each request to the remaining window so a single hung GET can't keep
    // the hook in `reviewing` past the deadline — without an AbortController the
    // loop's deadline check never re-runs while one fetch stalls.
    const controller = new AbortController()
    const timeoutID = setTimeout(() => controller.abort(), deadline - Date.now())
    try {
      const res = await fetch(`${API_URL}/uploads/${uploadID}`, {
        headers: { Authorization: `Bearer ${token}` },
        signal: controller.signal,
      })
      if (res.ok) {
        const data = await res.json()
        const status = data.moderation_status as string
        if (status === "approved" || status === "skipped") return { status: "approved" }
        // A quarantined upload is a legal-floor (CSAM-class) hit; surface it to
        // the user as a generic rejection — never as a distinct, tip-off state.
        if (status === "rejected" || status === "quarantined") {
          return {
            status: "rejected",
            code: data.moderation_code,
            reason: data.moderation_reason,
          }
        }
      }
    } catch {
      // Network blip or aborted request — retry on the next tick.
    } finally {
      clearTimeout(timeoutID)
    }
    const sleepMs = Math.min(POLL_INTERVAL_MS, deadline - Date.now())
    if (sleepMs > 0) await new Promise((r) => setTimeout(r, sleepMs))
  }
  return { status: "pending" } // still under review — do NOT fail open
}

function formatBytes(bytes: number): string {
  /* v8 ignore next 2 — KB branch unreachable: all category limits are ≥ 1 MB */
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(0)} MB`
  return `${(bytes / 1024).toFixed(0)} KB`
}

/**
 * Provides an `upload` function that handles the full 3-step upload flow:
 * 1. POST /uploads/request → get an upload URL
 * 2. PUT the file to that URL
 * 3. POST /uploads/{id}/confirm → finalize and get the public URL
 */
export function useUpload(token: string | undefined, opts?: { pollTimeoutMs?: number }) {
  const pollTimeoutMs = opts?.pollTimeoutMs ?? POLL_TIMEOUT_MS
  const [uploading, setUploading] = useState(false)
  const [reviewing, setReviewing] = useState(false)
  const [error, setError] = useState("")
  const [rejection, setRejection] = useState<UploadRejection | null>(null)
  const [pendingReview, setPendingReview] = useState(false)

  async function upload(
    file: File,
    category: UploadCategory,
  ): Promise<UploadResult | null> {
    if (!token) return null

    const maxSize = MAX_SIZES[category]
    if (file.size > maxSize) {
      setError(`File is too large. Maximum size is ${formatBytes(maxSize)}.`)
      return null
    }

    if (!ALLOWED_TYPES[category].includes(file.type)) {
      setError(`File type not allowed. Accepted: ${ALLOWED_TYPES[category].join(", ")}.`)
      return null
    }

    setUploading(true)
    setError("")
    setRejection(null)
    setPendingReview(false)

    try {
      // Step 1: Request upload URL.
      const reqRes = await fetch(`${API_URL}/uploads/request`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          category,
          filename: file.name,
          content_type: file.type,
          size_bytes: file.size,
        }),
      })
      if (!reqRes.ok) {
        const data = await reqRes.json().catch(() => ({}))
        setError(data.error || "Failed to request upload.")
        return null
      }
      const { upload_id, upload_url } = await reqRes.json()

      // Step 2: Upload the file.
      const putRes = await fetch(upload_url, {
        method: "PUT",
        body: file,
      })
      if (!putRes.ok) {
        setError("Failed to upload file.")
        return null
      }

      // Step 3: Confirm the upload.
      const confirmRes = await fetch(`${API_URL}/uploads/${upload_id}/confirm`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!confirmRes.ok) {
        setError("Failed to confirm upload.")
        return null
      }
      const result: UploadResult = await confirmRes.json()

      // Step 4: Poll the moderation outcome. The image pipeline runs async
      // in the worker; normally it completes within ~1s. Rejections surface as
      // a structured `rejection` value (not an `error` string) so consumers can
      // render the dedicated localized modal. A `pending` verdict (the poll
      // window elapsed while the server still held the image — e.g. a vendor
      // outage) surfaces as `pendingReview`: the image is NOT returned, because
      // the server's attach-time gate would reject it.
      if (file.type.startsWith("image/")) {
        setReviewing(true)
        try {
          const verdict = await pollModeration(upload_id, token, pollTimeoutMs)
          if (verdict.status === "rejected") {
            setRejection({ code: verdict.code ?? "unknown", reason: verdict.reason })
            return null
          }
          if (verdict.status === "pending") {
            setPendingReview(true)
            return null
          }
        } finally {
          setReviewing(false)
        }
      }

      return result
    } catch {
      setError("Upload failed.")
      return null
    } finally {
      setUploading(false)
    }
  }

  return {
    upload,
    uploading,
    reviewing,
    error,
    rejection,
    pendingReview,
    clearRejection: () => setRejection(null),
    clearPendingReview: () => setPendingReview(false),
  }
}
