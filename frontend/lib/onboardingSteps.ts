import type { MyProfile } from "@/contexts/ProfileContext"

export type OnboardingStep = "photo" | "bio" | "interests" | "location"

export const ONBOARDING_STEPS: OnboardingStep[] = ["photo", "bio", "interests", "location"]

function isStepDone(profile: MyProfile, step: OnboardingStep): boolean {
  switch (step) {
    case "photo":     return !!profile.avatar_url
    case "bio":       return !!profile.bio
    case "interests": return profile.interests.length > 0
    case "location":  return !!profile.location_text
  }
}

export function incompleteSteps(profile: MyProfile): OnboardingStep[] {
  return ONBOARDING_STEPS.filter((s) => !isStepDone(profile, s))
}

export function firstIncompleteStep(profile: MyProfile): OnboardingStep | null {
  return incompleteSteps(profile)[0] ?? null
}

/**
 * Returns the next onboarding route after completing `done`.
 * Excludes `done` from consideration and skips steps already filled in `profile`.
 * Returns "/" when no steps remain (caller should mark onboarded before navigating).
 */
export function nextStepAfter(profile: MyProfile, done: OnboardingStep): string {
  const remaining = ONBOARDING_STEPS.filter((s) => s !== done && !isStepDone(profile, s))
  return remaining.length > 0 ? `/onboarding/${remaining[0]}` : "/"
}
