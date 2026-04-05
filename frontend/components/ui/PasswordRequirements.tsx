export const PASSWORD_RULES = [
  { label: "At least 8 characters", test: (p: string) => p.length >= 8 },
  { label: "One uppercase letter",  test: (p: string) => /[A-Z]/.test(p) },
  { label: "One lowercase letter",  test: (p: string) => /[a-z]/.test(p) },
  { label: "One number",            test: (p: string) => /[0-9]/.test(p) },
]

export default function PasswordRequirements({ password }: { password: string }) {
  if (!password) return null
  return (
    <ul className="flex flex-col gap-1 pl-0.5" aria-label="Password requirements">
      {PASSWORD_RULES.map(({ label, test }) => {
        const met = test(password)
        return (
          <li
            key={label}
            className={`flex items-center gap-2 text-xs ${met ? "text-green-400" : "text-gray-500"}`}
          >
            <svg
              width="12"
              height="12"
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
            {label}
          </li>
        )
      })}
    </ul>
  )
}
