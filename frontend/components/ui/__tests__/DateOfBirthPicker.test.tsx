import { describe, it, expect } from "vitest"
import { render } from "@testing-library/react"
import { NextIntlClientProvider } from "next-intl"
import DateOfBirthPicker from "@/components/ui/DateOfBirthPicker"

function renderAt(locale: "es" | "en" | "pt") {
  return render(
    <NextIntlClientProvider locale={locale} messages={{}}>
      <DateOfBirthPicker
        value=""
        onChange={() => {}}
        labels={{ day: "DD", month: "MM", year: "YYYY" }}
      />
    </NextIntlClientProvider>,
  )
}

describe("DateOfBirthPicker locale-aware order", () => {
  it("renders DD/MM/YYYY for Spanish", () => {
    const { container } = renderAt("es")
    const labels = Array.from(container.querySelectorAll("select")).map(
      (s) => s.getAttribute("aria-label"),
    )
    expect(labels).toEqual(["DD", "MM", "YYYY"])
  })

  it("renders DD/MM/YYYY for Portuguese", () => {
    const { container } = renderAt("pt")
    const labels = Array.from(container.querySelectorAll("select")).map(
      (s) => s.getAttribute("aria-label"),
    )
    expect(labels).toEqual(["DD", "MM", "YYYY"])
  })

  it("renders MM/DD/YYYY for English (US convention)", () => {
    const { container } = renderAt("en")
    const labels = Array.from(container.querySelectorAll("select")).map(
      (s) => s.getAttribute("aria-label"),
    )
    expect(labels).toEqual(["MM", "DD", "YYYY"])
  })
})
