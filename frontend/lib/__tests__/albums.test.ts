import { describe, it, expect, beforeEach } from "vitest"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { albumsApi, absoluteAlbumURL } from "@/lib/albums"

const API = "http://localhost:8080"

describe("albumsApi", () => {
  let lastAuth: string | null
  let lastBody: unknown

  beforeEach(() => {
    lastAuth = null
    lastBody = null
  })

  it("sends the bearer token and parses the JSON response", async () => {
    server.use(
      http.get(`${API}/albums/me`, ({ request }) => {
        lastAuth = request.headers.get("Authorization")
        return HttpResponse.json([{ id: "a1", name: "Mine" }])
      }),
    )
    const albums = await albumsApi.listMine("tok-123")
    expect(lastAuth).toBe("Bearer tok-123")
    expect(albums).toEqual([{ id: "a1", name: "Mine" }])
  })

  it("sends a JSON body with Content-Type on create", async () => {
    server.use(
      http.post(`${API}/albums`, async ({ request }) => {
        lastBody = await request.json()
        expect(request.headers.get("Content-Type")).toBe("application/json")
        return HttpResponse.json({ id: "a2", name: "New" })
      }),
    )
    const album = await albumsApi.create("tok", "New", "desc")
    expect(lastBody).toEqual({ name: "New", description: "desc" })
    expect(album.id).toBe("a2")
  })

  it("resolves on 204 responses with no body", async () => {
    server.use(
      http.delete(`${API}/albums/a1`, () => new HttpResponse(null, { status: 204 })),
    )
    await expect(albumsApi.remove("tok", "a1")).resolves.toBeUndefined()
  })

  it("throws the backend error detail when present", async () => {
    server.use(
      http.post(`${API}/albums/a1/grants/request`, () =>
        HttpResponse.json({ error: "grant already exists" }, { status: 409 }),
      ),
    )
    await expect(albumsApi.requestAccess("tok", "a1")).rejects.toThrow("grant already exists")
  })

  it("falls back to the HTTP status when the error body is not JSON", async () => {
    server.use(
      http.get(`${API}/albums/missing`, () =>
        new HttpResponse("nope", { status: 404, statusText: "Not Found" }),
      ),
    )
    await expect(albumsApi.get("tok", "missing")).rejects.toThrow("404 Not Found")
  })

  it("passes the expiry preset through on invite and share-in-chat", async () => {
    const bodies: unknown[] = []
    server.use(
      http.post(`${API}/albums/a1/grants/invite`, async ({ request }) => {
        bodies.push(await request.json())
        return HttpResponse.json({ id: "g1" })
      }),
      http.post(`${API}/albums/a1/share-in-chat`, async ({ request }) => {
        bodies.push(await request.json())
        return HttpResponse.json({ album: { id: "a1" }, grant: { id: "g2" } })
      }),
    )
    await albumsApi.invite("tok", "a1", "user-9", "7d")
    await albumsApi.shareInChat("tok", "a1", "room-1")
    expect(bodies).toEqual([
      { grantee_id: "user-9", expires_in: "7d" },
      { room_id: "room-1", expires_in: "none" },
    ])
  })
})

describe("absoluteAlbumURL", () => {
  it("prefixes server-relative photo URLs with the API origin", () => {
    expect(absoluteAlbumURL("/albums/a1/photos/u1/file")).toBe(
      `${API}/albums/a1/photos/u1/file`,
    )
  })
})
