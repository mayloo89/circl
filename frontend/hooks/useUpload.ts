"use client"

import { useState } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export interface UploadResult {
  upload_id: string
  storage_key: string
  url: string
}

type UploadCategory = "avatar" | "chat-attachment" | "gallery"

const MAX_SIZES: Record<UploadCategory, number> = {
  avatar: 5 * 1024 * 1024,
  "chat-attachment": 50 * 1024 * 1024,
  gallery: 10 * 1024 * 1024,
}

const ALLOWED_TYPES: Record<UploadCategory, string[]> = {
  avatar: ["image/jpeg", "image/png", "image/webp"],
  "chat-attachment": ["image/jpeg", "image/png", "image/webp", "image/gif", "video/mp4", "video/quicktime", "application/pdf"],
  gallery: ["image/jpeg", "image/png", "image/webp"],
}

// pollModeration GETs the upload row until moderation completes or the
// timeout elapses. Returns the final verdict so the caller can show the
// rejection reason. Failing open ("approved") on poll error is intentional
// — a flaky API call should not look like a content rejection to the user.
async function pollModeration(
  uploadID: string,
  token: string,
): Promise<{ status: "approved" | "rejected" | "pending"; reason?: string }> {
  const deadline = Date.now() + 5000
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${API_URL}/uploads/${uploadID}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.ok) {
        const data = await res.json()
        const status = data.moderation_status as string
        if (status === "approved" || status === "skipped") return { status: "approved" }
        if (status === "rejected") {
          return { status: "rejected", reason: data.moderation_reason }
        }
      }
    } catch {
      // Network blip — retry on the next tick.
    }
    await new Promise((r) => setTimeout(r, 500))
  }
  return { status: "approved" } // timeout → fail open
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
export function useUpload(token: string | undefined) {
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState("")

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
      // in the worker; for the small images we accept here it completes
      // within ~1s. If moderation rejected the upload we surface the reason
      // in the user's locale rather than letting them stare at a broken URL.
      if (file.type.startsWith("image/")) {
        const verdict = await pollModeration(upload_id, token)
        if (verdict.status === "rejected") {
          setError(verdict.reason || "Upload was rejected by content moderation.")
          return null
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

  return { upload, uploading, error }
}
