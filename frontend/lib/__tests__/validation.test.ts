import { describe, it, expect } from "vitest"
import { loginSchema, registerSchema } from "@/lib/validation"

describe("loginSchema", () => {
  it("accepts valid credentials", () => {
    const result = loginSchema.safeParse({ email: "user@example.com", password: "secret" })
    expect(result.success).toBe(true)
    if (result.success) {
      expect(result.data.email).toBe("user@example.com")
      expect(result.data.password).toBe("secret")
    }
  })

  it("rejects an invalid email", () => {
    const result = loginSchema.safeParse({ email: "not-an-email", password: "secret" })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Please enter a valid email address")
    }
  })

  it("rejects an empty password", () => {
    const result = loginSchema.safeParse({ email: "user@example.com", password: "" })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Password is required")
    }
  })

  it("rejects missing fields", () => {
    const result = loginSchema.safeParse({})
    expect(result.success).toBe(false)
  })
})

describe("registerSchema", () => {
  it("accepts valid registration data", () => {
    const result = registerSchema.safeParse({
      email: "new@example.com",
      password: "strongpass",
      confirm: "strongpass",
    })
    expect(result.success).toBe(true)
  })

  it("rejects an invalid email", () => {
    const result = registerSchema.safeParse({
      email: "bad-email",
      password: "strongpass",
      confirm: "strongpass",
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Please enter a valid email address")
    }
  })

  it("rejects a password shorter than 8 characters", () => {
    const result = registerSchema.safeParse({
      email: "new@example.com",
      password: "short",
      confirm: "short",
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Password must be at least 8 characters")
    }
  })

  it("rejects mismatched passwords", () => {
    const result = registerSchema.safeParse({
      email: "new@example.com",
      password: "strongpass",
      confirm: "different",
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Passwords do not match")
    }
  })
})
