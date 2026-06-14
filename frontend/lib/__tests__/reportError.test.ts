import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { reportClientError } from "@/lib/reportError"

describe("reportClientError", () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal("fetch", fetchMock)
  })
  afterEach(() => vi.unstubAllGlobals())

  it("posts an Error to /client-errors with message, stack, url and kind", () => {
    reportClientError({ error: new Error("boom"), kind: "error" })

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, opts] = fetchMock.mock.calls[0]
    expect(String(url)).toContain("/client-errors")
    expect(opts.method).toBe("POST")
    expect(opts.keepalive).toBe(true)

    const body = JSON.parse(opts.body as string)
    expect(body.message).toBe("boom")
    expect(body.kind).toBe("error")
    expect(typeof body.stack).toBe("string")
    expect(typeof body.url).toBe("string")
  })

  it("stringifies non-Error reasons (e.g. rejected promises)", () => {
    reportClientError({ error: "plain rejection", kind: "unhandledrejection" })
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string)
    expect(body.message).toBe("plain rejection")
    expect(body.stack).toBeUndefined()
  })

  it("never throws when the network request rejects", () => {
    fetchMock.mockReturnValue(Promise.reject(new Error("network down")))
    expect(() =>
      reportClientError({ error: new Error("x"), kind: "boundary" }),
    ).not.toThrow()
  })
})
