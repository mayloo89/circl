import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import SearchBar from "@/components/contacts/SearchBar"

const results = [
  { id: "u1", username: "ada", email: "ada@example.com", display_name: "Ada", avatar_url: "" },
  { id: "u2", username: "", email: "bo@example.com", display_name: "", avatar_url: "" },
]

describe("SearchBar", () => {
  it("reports typed input through onChange", () => {
    const onChange = vi.fn()
    renderWithIntl(<SearchBar value="" onChange={onChange} results={[]} onAdd={vi.fn()} />)
    fireEvent.change(screen.getByPlaceholderText("Search by name or email…"), {
      target: { value: "ada" },
    })
    expect(onChange).toHaveBeenCalledWith("ada")
  })

  it("hides the result list when there are no matches", () => {
    renderWithIntl(<SearchBar value="z" onChange={vi.fn()} results={[]} onAdd={vi.fn()} />)
    expect(screen.queryByRole("listitem")).toBeNull()
  })

  it("adds a user by id from the result row", () => {
    const onAdd = vi.fn()
    renderWithIntl(<SearchBar value="a" onChange={vi.fn()} results={results} onAdd={onAdd} />)
    fireEvent.click(screen.getAllByRole("button", { name: "Add" })[0])
    expect(onAdd).toHaveBeenCalledWith("u1")
  })

  it("navigates by username, falling back to id, and labels each row", () => {
    const onNavigate = vi.fn()
    renderWithIntl(
      <SearchBar value="a" onChange={vi.fn()} results={results} onAdd={vi.fn()} onNavigate={onNavigate} />,
    )
    fireEvent.click(screen.getByRole("button", { name: "View profile of Ada" }))
    fireEvent.click(screen.getByRole("button", { name: "View profile of bo@example.com" }))
    expect(onNavigate).toHaveBeenNthCalledWith(1, "ada")
    expect(onNavigate).toHaveBeenNthCalledWith(2, "u2")
  })

  it("disables navigation when no handler is provided", () => {
    renderWithIntl(<SearchBar value="a" onChange={vi.fn()} results={results} onAdd={vi.fn()} />)
    expect(screen.getByRole("button", { name: "View profile of Ada" })).toBeDisabled()
  })
})
