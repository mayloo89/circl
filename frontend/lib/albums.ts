// Client-side API helpers for the private-albums feature. Every function
// is a thin wrapper around fetch — auth is the caller's responsibility
// (pass the access token from useSession). Pure functions, no hooks.

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export interface Album {
  id: string
  owner_id: string
  name: string
  description: string
  cover_upload_id?: string | null
  photo_count: number
  created_at: string
  updated_at: string
  cover_url?: string
  role?: "owner" | "viewer" | ""
}

export interface AlbumPhoto {
  upload_id: string
  album_id: string
  position: number
  added_at: string
  filename: string
  content_type: string
  url: string
}

export type GrantStatus = "pending" | "active" | "denied" | "revoked"
export type GrantSource = "invite" | "request" | "chat"

export interface Grant {
  id: string
  album_id: string
  granter_id: string
  grantee_id: string
  status: GrantStatus
  source: GrantSource
  requested_at: string
  granted_at?: string | null
  revoked_at?: string | null
  counterparty?: string
}

async function authedJSON<T>(token: string, path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      ...(init.headers ?? {}),
      Authorization: `Bearer ${token}`,
      ...(init.body ? { "Content-Type": "application/json" } : {}),
    },
  })
  if (!res.ok) {
    let detail = ""
    try {
      const data = (await res.json()) as { error?: string }
      detail = data.error ?? ""
    } catch {
      // ignore
    }
    throw new Error(detail || `${res.status} ${res.statusText}`)
  }
  if (res.status === 204) return undefined as unknown as T
  return (await res.json()) as T
}

export const albumsApi = {
  listMine: (token: string) => authedJSON<Album[]>(token, "/albums/me"),
  listSharedWithMe: (token: string) => authedJSON<Album[]>(token, "/albums/shared-with-me"),
  get: (token: string, id: string) => authedJSON<Album>(token, `/albums/${id}`),
  create: (token: string, name: string, description: string) =>
    authedJSON<Album>(token, "/albums", { method: "POST", body: JSON.stringify({ name, description }) }),
  update: (token: string, id: string, patch: Partial<{ name: string; description: string; cover_upload_id: string; clear_cover: boolean }>) =>
    authedJSON<Album>(token, `/albums/${id}`, { method: "PATCH", body: JSON.stringify(patch) }),
  remove: (token: string, id: string) =>
    authedJSON<void>(token, `/albums/${id}`, { method: "DELETE" }),

  listPhotos: (token: string, id: string) => authedJSON<AlbumPhoto[]>(token, `/albums/${id}/photos`),
  addPhoto: (token: string, id: string, uploadID: string) =>
    authedJSON<void>(token, `/albums/${id}/photos`, { method: "POST", body: JSON.stringify({ upload_id: uploadID }) }),
  removePhoto: (token: string, id: string, uploadID: string) =>
    authedJSON<void>(token, `/albums/${id}/photos/${uploadID}`, { method: "DELETE" }),

  listGrants: (token: string, id: string) => authedJSON<Grant[]>(token, `/albums/${id}/grants`),
  invite: (token: string, id: string, granteeID: string) =>
    authedJSON<Grant>(token, `/albums/${id}/grants/invite`, { method: "POST", body: JSON.stringify({ grantee_id: granteeID }) }),
  requestAccess: (token: string, id: string) =>
    authedJSON<Grant>(token, `/albums/${id}/grants/request`, { method: "POST" }),
  accept: (token: string, grantID: string) =>
    authedJSON<Grant>(token, `/albums/grants/${grantID}/accept`, { method: "POST" }),
  deny: (token: string, grantID: string) =>
    authedJSON<Grant>(token, `/albums/grants/${grantID}/deny`, { method: "POST" }),
  revoke: (token: string, grantID: string) =>
    authedJSON<Grant>(token, `/albums/grants/${grantID}/revoke`, { method: "POST" }),

  shareInChat: (token: string, id: string, roomID: string) =>
    authedJSON<{ album: Album; grant: Grant }>(token, `/albums/${id}/share-in-chat`, {
      method: "POST",
      body: JSON.stringify({ room_id: roomID }),
    }),
}

// Absolute URL for the photo stream endpoint. The relative `url` field on
// AlbumPhoto starts with `/albums/...` (server-relative) — callers turn it
// into a full URL via this helper.
export function absoluteAlbumURL(relativeURL: string): string {
  return `${API_URL}${relativeURL}`
}
