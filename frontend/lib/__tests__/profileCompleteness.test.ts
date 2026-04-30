import { describe, expect, it } from "vitest"
import { computeCompleteness } from "../profileCompleteness"

describe("computeCompleteness", () => {
  it("returns 100% when all fields are present", () => {
    const result = computeCompleteness({
      avatar_url: "https://example.com/avatar.jpg",
      bio: "Hello world",
      interests: ["music", "travel"],
      date_of_birth: "1990-01-01",
      location_text: "Paris, France",
    })
    expect(result.percent).toBe(100)
    expect(result.missing).toHaveLength(0)
  })

  it("returns 0% when all fields are missing", () => {
    const result = computeCompleteness({})
    expect(result.percent).toBe(0)
    expect(result.missing).toEqual(["avatar", "bio", "interests", "birthdate", "location"])
  })

  it("accounts for correct weights per field", () => {
    expect(computeCompleteness({ avatar_url: "x", bio: "x", interests: ["x"], date_of_birth: "x" }).percent).toBe(80)
    expect(computeCompleteness({ location_text: "x" }).percent).toBe(20)
    expect(computeCompleteness({ date_of_birth: "x" }).percent).toBe(10)
    expect(computeCompleteness({ bio: "x", interests: ["x"] }).percent).toBe(40)
  })

  it("treats empty string bio as missing", () => {
    const result = computeCompleteness({ bio: "   " })
    expect(result.missing).toContain("bio")
  })

  it("treats empty interests array as missing", () => {
    const result = computeCompleteness({ interests: [] })
    expect(result.missing).toContain("interests")
  })

  it("treats empty location_text as missing", () => {
    const result = computeCompleteness({ location_text: "" })
    expect(result.missing).toContain("location")
  })
})
