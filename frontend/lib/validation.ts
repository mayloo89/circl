import { z } from "zod"

export const loginSchema = z.object({
  email: z.string().email("Please enter a valid email address"),
  password: z.string().min(1, "Password is required"),
})

function isAtLeast18(dob: string): boolean {
  const dobDate = new Date(dob)
  const today = new Date()
  const cutoff = new Date(today.getFullYear() - 18, today.getMonth(), today.getDate())
  return dobDate <= cutoff
}

export const registerSchema = z
  .object({
    email: z.string().email("Please enter a valid email address"),
    password: z
      .string()
      .min(8, "Password must be at least 8 characters")
      .regex(/[A-Z]/, "Password must contain at least one uppercase letter")
      .regex(/[a-z]/, "Password must contain at least one lowercase letter")
      .regex(/[0-9]/, "Password must contain at least one digit"),
    confirm: z.string(),
    username: z
      .string()
      .regex(/^[a-z0-9_]{3,30}$/, "Username must be 3–30 characters: lowercase letters, digits, or underscores"),
    date_of_birth: z
      .string()
      .min(1, "Date of birth is required")
      .refine(isAtLeast18, "You must be at least 18 years old"),
  })
  .refine((data) => data.password === data.confirm, {
    message: "Passwords do not match",
    path: ["confirm"],
  })

export type LoginInput = z.infer<typeof loginSchema>
export type RegisterInput = z.infer<typeof registerSchema>
