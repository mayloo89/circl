import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import PhotoGallery, { type ProfilePhoto } from "@/components/profile/PhotoGallery"

const photos: ProfilePhoto[] = [
  { id: "p1", url: "/a.jpg" },
  { id: "p2", url: "/b.jpg" },
  { id: "p3", url: "/c.jpg" },
]

describe("PhotoGallery — view mode", () => {
  it("renders nothing when there are no photos", () => {
    const { container } = renderWithIntl(<PhotoGallery photos={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it("opens a photo via onPhotoClick", () => {
    const onPhotoClick = vi.fn()
    renderWithIntl(<PhotoGallery photos={photos} onPhotoClick={onPhotoClick} />)
    fireEvent.click(screen.getAllByRole("button")[0])
    expect(onPhotoClick).toHaveBeenCalledWith("/a.jpg")
  })
})

describe("PhotoGallery — editable", () => {
  it("shows the photo count and an upload slot", () => {
    const onAdd = vi.fn()
    renderWithIntl(<PhotoGallery photos={photos} editable onAdd={onAdd} maxPhotos={6} />)
    expect(screen.getByText("3/6")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Add photo" }))
    expect(onAdd).toHaveBeenCalledOnce()
  })

  it("confirms before deleting a photo", () => {
    const onDelete = vi.fn()
    renderWithIntl(<PhotoGallery photos={photos} editable onDelete={onDelete} />)
    fireEvent.click(screen.getAllByRole("button", { name: "Delete photo?" })[0])
    expect(screen.getByText("Delete photo?")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Delete" }))
    expect(onDelete).toHaveBeenCalledWith("p1")
  })

  it("cancels a pending delete", () => {
    const onDelete = vi.fn()
    renderWithIntl(<PhotoGallery photos={photos} editable onDelete={onDelete} />)
    fireEvent.click(screen.getAllByRole("button", { name: "Delete photo?" })[0])
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
    expect(onDelete).not.toHaveBeenCalled()
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull()
  })

  it("reorders a photo forward with the reordered id list", async () => {
    const onReorder = vi.fn().mockResolvedValue(undefined)
    renderWithIntl(<PhotoGallery photos={photos} editable onReorder={onReorder} />)
    fireEvent.click(screen.getAllByRole("button", { name: "Move forward" })[0])
    await waitFor(() => expect(onReorder).toHaveBeenCalledWith(["p2", "p1", "p3"]))
  })

  it("promotes the second photo to main", async () => {
    const onReorder = vi.fn().mockResolvedValue(undefined)
    renderWithIntl(<PhotoGallery photos={photos} editable onReorder={onReorder} />)
    fireEvent.click(screen.getByRole("button", { name: "Make main" }))
    await waitFor(() => expect(onReorder).toHaveBeenCalledWith(["p2", "p1", "p3"]))
  })

  it("hides reorder controls when no reorder handler is given", () => {
    renderWithIntl(<PhotoGallery photos={photos} editable />)
    expect(screen.queryByRole("button", { name: "Move forward" })).toBeNull()
  })

  it("renders an error message", () => {
    renderWithIntl(<PhotoGallery photos={photos} editable error="Upload failed" />)
    expect(screen.getByText("Upload failed")).toBeInTheDocument()
  })

  it("disables the upload slot while uploading", () => {
    renderWithIntl(<PhotoGallery photos={[]} editable uploading onAdd={vi.fn()} />)
    expect(screen.getByRole("button", { name: "Add photo" })).toBeDisabled()
  })
})
