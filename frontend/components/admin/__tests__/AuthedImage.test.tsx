import { describe, it, expect, beforeEach, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import AuthedImage from "@/components/admin/AuthedImage"

const SRC = "http://localhost:8080/admin/moderation/u1/image"

describe("AuthedImage", () => {
  beforeEach(() => {
    // jsdom has no URL.createObjectURL; stub the blob-URL lifecycle.
    URL.createObjectURL = vi.fn().mockReturnValue("blob:fake-url")
    URL.revokeObjectURL = vi.fn()
  })

  it("fetches with the Bearer header and renders the blob image", async () => {
    let auth: string | null = null
    server.use(
      http.get(SRC, ({ request }) => {
        auth = request.headers.get("Authorization")
        return new HttpResponse(new Blob(["img"]), { status: 200 })
      }),
    )
    render(<AuthedImage src={SRC} token="admin-tok" alt="Rejected upload" />)
    const img = await screen.findByRole("img")
    expect(img).toHaveAttribute("src", "blob:fake-url")
    expect(img).toHaveAttribute("alt", "Rejected upload")
    expect(auth).toBe("Bearer admin-tok")
  })

  it("renders a placeholder when the fetch is unauthorized", async () => {
    server.use(http.get(SRC, () => new HttpResponse(null, { status: 401 })))
    render(<AuthedImage src={SRC} token="bad" alt="Rejected upload" />)
    await waitFor(() => expect(screen.getByText("N/A")).toBeInTheDocument())
  })

  it("revokes the blob URL on unmount", async () => {
    server.use(http.get(SRC, () => new HttpResponse(new Blob(["img"]), { status: 200 })))
    const { unmount } = render(<AuthedImage src={SRC} token="t" alt="x" />)
    await screen.findByRole("img")
    unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:fake-url")
  })
})
