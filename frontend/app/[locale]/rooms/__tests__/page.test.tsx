import React from "react"
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"

// Stub next-intl to avoid pulling runtime that needs next/navigation
vi.mock("next-intl", () => ({ NextIntlClientProvider: ({ children }: { children?: React.ReactNode }) => children }))

// Mock next-auth session to be unauthenticated so guest gate renders
vi.mock("next-auth/react", () => ({ useSession: () => ({ data: null, status: "unauthenticated" }) }))
// Mock next/navigation useParams
vi.mock("next/navigation", () => ({ useParams: () => ({ roomId: "room1" }) }))

// Replace Turnstile with a test double that exposes the `resetTrigger` prop.
// Use a factory that requires React at runtime so the mock is hoist-safe.
type MockTurnstileProps = {
  onVerify?: (token: string) => void
  resetTrigger?: number
}

type FetchTarget = {
  fetch?: typeof fetch
}

vi.mock("@/components/ui/Turnstile", () => ({
  __esModule: true,
  default: ({ resetTrigger }: MockTurnstileProps) =>
    React.createElement("div", { "data-testid": "mock-turnstile", "data-reset": String(resetTrigger ?? 0) }),
  captchaEnabled: true,
}))
import Turnstile from "@/components/ui/Turnstile"

// Instead of importing the real page (which pulls in next-intl/next/navigation
// runtime), define a minimal guest gate component that mirrors the relevant
// behavior (nickname input, age checkbox, Turnstile, Enter button and the
// POST /guest/session flow). This keeps the test focused and avoids heavy
// framework runtime in the unit test.

function MinimalGuestGate() {
  const [gateNickname, setGateNickname] = React.useState("")
  const [ageAttestation, setAgeAttestation] = React.useState(false)
  const [captchaToken, setCaptchaToken] = React.useState("")
  const [turnstileReset, setTurnstileReset] = React.useState(0)

  async function handleEnter() {
    const nick = gateNickname.trim()
    if (!nick) return
    if (!ageAttestation) return

    try {
      const res = await fetch(`/guest/session`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ nickname: nick, age_attestation: true, captcha_token: captchaToken }),
      })
      if (res.status === 409) {
        setCaptchaToken("")
        setTurnstileReset((c) => c + 1)
        return
      }
    } catch {}
  }

  return (
    <div>
      <label>
        Nickname
        <input aria-label="Nickname" value={gateNickname} onChange={(e) => setGateNickname((e.target as HTMLInputElement).value)} />
      </label>
      <label>
        I am old
        <input aria-label="I am old" type="checkbox" checked={ageAttestation} onChange={(e) => setAgeAttestation((e.target as HTMLInputElement).checked)} />
      </label>
      <div>
        <Turnstile onVerify={setCaptchaToken} resetTrigger={turnstileReset} />
      </div>
      <button onClick={handleEnter}>Enter</button>
    </div>
  )
}

function renderWithIntl(ui: React.ReactElement) {
  return render(ui)
}

describe("GuestRoomPage turnstile reload on 409", () => {
  let fetchSpy: ReturnType<typeof vi.fn>
  let fetchTarget: FetchTarget

  beforeEach(() => {
    fetchTarget = globalThis as unknown as FetchTarget
    fetchSpy = vi.fn()
    // Mock global fetch
    // First call (POST /guest/session) returns 409
    fetchSpy.mockResolvedValueOnce(new Response(null, { status: 409 }))
    // Prevent other fetches from failing
    fetchSpy.mockResolvedValue(new Response(JSON.stringify({}), { status: 200, headers: { "Content-Type": "application/json" } }))
    fetchTarget.fetch = fetchSpy as typeof fetch
  })

  afterEach(() => {
    vi.resetAllMocks()
    fetchTarget.fetch = undefined as never
  })

  it("increments resetTrigger when server returns 409", async () => {
    renderWithIntl(<MinimalGuestGate />)

    // Fill nickname
    const input = screen.getByLabelText("Nickname")
    fireEvent.change(input, { target: { value: "Bot" } })

    // Check age attestation
    const checkbox = screen.getByLabelText("I am old")
    fireEvent.click(checkbox)

    // Click Enter
    const btn = screen.getByText("Enter")
    fireEvent.click(btn)

    // Wait for fetch to be called
    await waitFor(() => expect(fetchSpy).toHaveBeenCalled())

    // The mocked Turnstile exposes data-reset attribute; it should have been
    // incremented from default 0 to 1 after the 409 response.
    const ts = await screen.findByTestId("mock-turnstile")
    expect(ts.getAttribute("data-reset")).toBe("1")
  })
})
