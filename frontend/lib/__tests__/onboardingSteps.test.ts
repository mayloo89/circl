import { describe, it, expect } from "vitest"
import { incompleteSteps, firstIncompleteStep, nextStepAfter, ONBOARDING_STEPS } from "@/lib/onboardingSteps"
import type { MyProfile } from "@/contexts/ProfileContext"

function profile(overrides: Partial<MyProfile> = {}): MyProfile {
  return {
    username: "alice",
    display_name: "Alice",
    avatar_url: "",
    bio: "",
    interests: [],
    location_text: "",
    onboarded_at: null,
    ...overrides,
  }
}

describe("incompleteSteps", () => {
  it("returns all steps for an empty profile", () => {
    expect(incompleteSteps(profile())).toEqual(ONBOARDING_STEPS)
  })

  it("returns nothing for a complete profile", () => {
    const full = profile({
      avatar_url: "http://cdn/a.jpg",
      bio: "hi",
      interests: ["music"],
      location_text: "Buenos Aires",
    })
    expect(incompleteSteps(full)).toEqual([])
  })

  it("omits only the filled steps", () => {
    const p = profile({ avatar_url: "http://cdn/a.jpg", interests: ["music"] })
    expect(incompleteSteps(p)).toEqual(["bio", "location"])
  })
})

describe("firstIncompleteStep", () => {
  it("returns the first remaining step in canonical order", () => {
    expect(firstIncompleteStep(profile({ avatar_url: "x" }))).toBe("bio")
  })

  it("returns null when everything is filled", () => {
    const full = profile({
      avatar_url: "x",
      bio: "x",
      interests: ["x"],
      location_text: "x",
    })
    expect(firstIncompleteStep(full)).toBeNull()
  })
})

describe("nextStepAfter", () => {
  it("skips the step just completed even if the profile is not yet updated", () => {
    expect(nextStepAfter(profile(), "photo")).toBe("/onboarding/bio")
  })

  it("skips steps already filled in the profile", () => {
    const p = profile({ bio: "hi", interests: ["music"] })
    expect(nextStepAfter(p, "photo")).toBe("/onboarding/location")
  })

  it("returns home when no steps remain", () => {
    const p = profile({ bio: "hi", interests: ["music"], location_text: "BA" })
    expect(nextStepAfter(p, "photo")).toBe("/")
  })
})
