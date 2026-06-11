import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import CreateAlbumDialog from "@/components/albums/CreateAlbumDialog"

const API = "http://localhost:8080"

const baseProps = {
  open: true,
  token: "tok",
  onClose: vi.fn(),
  onCreated: vi.fn(),
}

describe("CreateAlbumDialog", () => {
  it("renders nothing when closed", () => {
    renderWithIntl(<CreateAlbumDialog {...baseProps} open={false} />)
    expect(screen.queryByText("New album")).not.toBeInTheDocument()
  })

  it("disables save until a name is entered", () => {
    renderWithIntl(<CreateAlbumDialog {...baseProps} />)
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled()
    fireEvent.change(screen.getByLabelText("Album name"), { target: { value: "Trip" } })
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled()
  })

  it("creates the album and reports it to the caller", async () => {
    const onCreated = vi.fn()
    const onClose = vi.fn()
    let body: unknown
    server.use(
      http.post(`${API}/albums`, async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ id: "a1", name: "Trip" })
      }),
    )
    renderWithIntl(<CreateAlbumDialog {...baseProps} onCreated={onCreated} onClose={onClose} />)
    fireEvent.change(screen.getByLabelText("Album name"), { target: { value: "Trip" } })
    fireEvent.change(screen.getByLabelText(/description/i), { target: { value: "Summer" } })
    fireEvent.click(screen.getByRole("button", { name: "Save" }))

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith({ id: "a1", name: "Trip" }))
    expect(body).toEqual({ name: "Trip", description: "Summer" })
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("surfaces the backend error and stays open on failure", async () => {
    server.use(
      http.post(`${API}/albums`, () =>
        HttpResponse.json({ error: "album limit reached" }, { status: 422 }),
      ),
    )
    renderWithIntl(<CreateAlbumDialog {...baseProps} />)
    fireEvent.change(screen.getByLabelText("Album name"), { target: { value: "Trip" } })
    fireEvent.click(screen.getByRole("button", { name: "Save" }))

    expect(await screen.findByRole("alert")).toHaveTextContent("album limit reached")
    expect(baseProps.onCreated).not.toHaveBeenCalled()
  })
})
