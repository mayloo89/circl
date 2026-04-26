export interface ProfileFieldsForCompleteness {
  avatar_url?: string
  bio?: string
  interests?: string[]
  date_of_birth?: string
  location_text?: string
}

export type MissingField = "avatar" | "bio" | "interests" | "birthdate" | "location"

export interface CompletenessResult {
  percent: number
  missing: MissingField[]
}

const weights: Record<MissingField, number> = {
  avatar: 30,
  bio: 20,
  interests: 20,
  birthdate: 10,
  location: 20,
}

export function computeCompleteness(p: ProfileFieldsForCompleteness): CompletenessResult {
  const missing: MissingField[] = []
  if (!p.avatar_url) missing.push("avatar")
  if (!p.bio?.trim()) missing.push("bio")
  if (!p.interests?.length) missing.push("interests")
  if (!p.date_of_birth) missing.push("birthdate")
  if (!p.location_text?.trim()) missing.push("location")
  const earned = 100 - missing.reduce((sum, k) => sum + weights[k], 0)
  return { percent: earned, missing }
}
