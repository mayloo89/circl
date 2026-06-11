import { describe, it, expect, vi } from "vitest"
import { screen } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"

vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import AlbumCard from "@/components/albums/AlbumCard"
import type { Album } from "@/lib/albums"

function album(overrides: Partial<Album> = {}): Album {
  return {
    id: "a1",
    owner_id: "u1",
    name: "Trip photos",
    description: "",
    photo_count: 3,
    created_at: "2026-06-01T00:00:00Z",
    updated_at: "2026-06-01T00:00:00Z",
    ...overrides,
  }
}

describe("AlbumCard", () => {
  it("links to the album detail page", () => {
    renderWithIntl(<AlbumCard album={album()} />)
    expect(screen.getByRole("link")).toHaveAttribute("href", "/en/albums/a1")
    expect(screen.getByText("Trip photos")).toBeInTheDocument()
  })

  it("pluralizes the photo count", () => {
    renderWithIntl(<AlbumCard album={album({ photo_count: 3 })} />)
    expect(screen.getByText("3 photos")).toBeInTheDocument()
  })

  it("shows the empty-count copy", () => {
    renderWithIntl(<AlbumCard album={album({ photo_count: 0 })} />)
    expect(screen.getByText("No photos")).toBeInTheDocument()
  })

  it("shows the owner attribution on shared albums", () => {
    renderWithIntl(<AlbumCard album={album({ owner_name: "Bea" })} />)
    expect(screen.getByText("Bea")).toBeInTheDocument()
  })

  it("omits the owner row on own albums", () => {
    renderWithIntl(<AlbumCard album={album()} />)
    expect(screen.queryByText("Bea")).not.toBeInTheDocument()
  })
})
