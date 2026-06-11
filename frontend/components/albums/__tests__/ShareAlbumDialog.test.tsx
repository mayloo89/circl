import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import ShareAlbumDialog from "@/components/albums/ShareAlbumDialog"

const API = "http://localhost:8080"

const myAlbums = [
  { id: "a1", owner_id: "u1", name: "Trip", description: "", photo_count: 2, created_at: "", updated_at: "" },
  { id: "a2", owner_id: "u1", name: "Pets", description: "", photo_count: 5, created_at: "", updated_at: "" },
]

const baseProps = {
  open: true,
  token: "tok",
  roomID: "room-1",
  onClose: vi.fn(),
  onShared: vi.fn(),
}

function listMineHandler() {
  return http.get(`${API}/albums/me`, () => HttpResponse.json(myAlbums))
}

describe("ShareAlbumDialog", () => {
  it("lists the caller's albums", async () => {
    server.use(listMineHandler())
    renderWithIntl(<ShareAlbumDialog {...baseProps} />)
    expect(await screen.findByText("Trip")).toBeInTheDocument()
    expect(screen.getByText("Pets")).toBeInTheDocument()
  })

  it("shows the empty state when there are no albums", async () => {
    server.use(http.get(`${API}/albums/me`, () => HttpResponse.json([])))
    renderWithIntl(<ShareAlbumDialog {...baseProps} />)
    expect(await screen.findByText("You don't have any albums yet.")).toBeInTheDocument()
  })

  it("shares with the default no-expiry grant", async () => {
    const onShared = vi.fn()
    const onClose = vi.fn()
    let body: unknown
    server.use(
      listMineHandler(),
      http.post(`${API}/albums/a1/share-in-chat`, async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ album: myAlbums[0], grant: { id: "g1" } })
      }),
    )
    renderWithIntl(<ShareAlbumDialog {...baseProps} onShared={onShared} onClose={onClose} />)
    await screen.findByText("Trip")
    fireEvent.click(screen.getAllByRole("button", { name: "Share in chat" })[0])

    await waitFor(() => expect(onShared).toHaveBeenCalledOnce())
    expect(body).toEqual({ room_id: "room-1", expires_in: "none" })
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("applies the selected expiry preset to the grant", async () => {
    let body: unknown
    server.use(
      listMineHandler(),
      http.post(`${API}/albums/a1/share-in-chat`, async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ album: myAlbums[0], grant: { id: "g1" } })
      }),
    )
    renderWithIntl(<ShareAlbumDialog {...baseProps} />)
    await screen.findByText("Trip")
    fireEvent.click(screen.getByRole("button", { name: "24 hours" }))
    fireEvent.click(screen.getAllByRole("button", { name: "Share in chat" })[0])

    await waitFor(() => expect(body).toEqual({ room_id: "room-1", expires_in: "24h" }))
  })

  it("shows an error and stays open when sharing fails", async () => {
    server.use(
      listMineHandler(),
      http.post(`${API}/albums/a1/share-in-chat`, () =>
        HttpResponse.json({ error: "boom" }, { status: 500 }),
      ),
    )
    renderWithIntl(<ShareAlbumDialog {...baseProps} />)
    await screen.findByText("Trip")
    fireEvent.click(screen.getAllByRole("button", { name: "Share in chat" })[0])

    expect(await screen.findByRole("alert")).toHaveTextContent("Failed to share album.")
  })

  it("shows a load error when the album list cannot be fetched", async () => {
    server.use(http.get(`${API}/albums/me`, () => HttpResponse.json({}, { status: 500 })))
    renderWithIntl(<ShareAlbumDialog {...baseProps} />)
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't load albums.")
  })
})
