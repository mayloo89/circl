"use client"

import { useTranslations } from "next-intl"

// Test functions only — labels come from i18n translations
export const PASSWORD_RULES: Array<{ id: string; test: (p: string) => boolean }> = [
  { id: "minLength", test: (p: string) => p.length >= 8 },
  { id: "uppercase", test: (p: string) => /[A-Z]/.test(p) },
  { id: "lowercase", test: (p: string) => /[a-z]/.test(p) },
  { id: "number",    test: (p: string) => /[0-9]/.test(p) },
]

export default function PasswordRequirements({ password }: { password: string }) {
  const t = useTranslations("passwordRequirements")

  return (
    <ul className="flex flex-col gap-1 pl-0.5" aria-label="Password requirements">
      {PASSWORD_RULES.map(({ id, test }) => {
        const met = test(password)
        return (
          <li
            key={id}
            className={`flex items-center gap-2 text-xs ${met ? "text-green-400" : "text-gray-500"}`}
          >
            <svg
              className="h-3 w-3"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              {met ? (
                <polyline points="20 6 9 17 4 12" />
              ) : (
                <>
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </>
              )}
            </svg>
            {t(id as "minLength" | "uppercase" | "lowercase" | "number")}
          </li>
        )
      })}
    </ul>
  )
}
