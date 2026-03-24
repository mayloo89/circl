"use client"

import { useState } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export interface UploadResult {
  upload_id: string
  storage_key: string
  url: string
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
    category: "avatar" | "chat-attachment" | "profile-photo",
  ): Promise<UploadResult | null> {
    if (!token) return null
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
