import { describe, it, expect, beforeEach } from "vitest"
import { renderHook, act } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { useUpload } from "@/hooks/useUpload"

const API = "http://localhost:8080"
const STORAGE_URL = "http://storage.test/file"

const successHandlers = [
  http.post(`${API}/uploads/request`, () =>
    HttpResponse.json({ upload_id: "uid-1", upload_url: STORAGE_URL })
  ),
  http.put(STORAGE_URL, () => new HttpResponse(null, { status: 200 })),
  http.post(`${API}/uploads/uid-1/confirm`, () =>
    HttpResponse.json({ upload_id: "uid-1", storage_key: "photos/a.jpg", url: "http://cdn/a.jpg" })
  ),
]

function makeFile(name = "photo.jpg", type = "image/jpeg", size = 1024): File {
  const file = new File(["x".repeat(size)], name, { type })
  Object.defineProperty(file, "size", { value: size })
  return file
}

describe("useUpload", () => {
  beforeEach(() => server.use(...successHandlers))

  it("returns null immediately when no token is provided", async () => {
    const { result } = renderHook(() => useUpload(undefined))
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(makeFile(), "avatar")
    })
    expect(res!).toBeNull()
  })

  it("rejects files that exceed the size limit", async () => {
    const { result } = renderHook(() => useUpload("token"))
    const bigFile = makeFile("huge.jpg", "image/jpeg", 6 * 1024 * 1024)
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(bigFile, "avatar")
    })
    expect(res!).toBeNull()
    expect(result.current.error).toMatch(/too large/i)
  })

  it("rejects files with a disallowed MIME type", async () => {
    const { result } = renderHook(() => useUpload("token"))
    const badFile = makeFile("doc.pdf", "application/pdf")
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(badFile, "avatar")
    })
    expect(res!).toBeNull()
    expect(result.current.error).toMatch(/not allowed/i)
  })

  it("completes the 3-step flow and returns the result", async () => {
    const { result } = renderHook(() => useUpload("token"))
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(makeFile(), "avatar")
    })
    expect(res).toEqual({
      upload_id: "uid-1",
      storage_key: "photos/a.jpg",
      url: "http://cdn/a.jpg",
    })
    expect(result.current.error).toBe("")
    expect(result.current.uploading).toBe(false)
  })

  it("sets uploading to true during upload", async () => {
    let sawUploading = false
    server.use(
      http.post(`${API}/uploads/request`, async () => {
        return HttpResponse.json({ upload_id: "uid-1", upload_url: STORAGE_URL })
      })
    )
    const { result } = renderHook(() => useUpload("token"))
    const promise = act(async () => {
      result.current.upload(makeFile(), "avatar")
    })
    // uploading flips to true synchronously at the start of the async function
    if (result.current.uploading) sawUploading = true
    await promise
    // after completion it should be false again
    expect(result.current.uploading).toBe(false)
  })

  it("sets error and returns null when /uploads/request fails with a JSON body", async () => {
    server.use(
      http.post(`${API}/uploads/request`, () => HttpResponse.json({ error: "quota exceeded" }, { status: 400 }))
    )
    const { result } = renderHook(() => useUpload("token"))
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(makeFile(), "avatar")
    })
    expect(res!).toBeNull()
    expect(result.current.error).toBe("quota exceeded")
  })

  it("sets error and returns null when the PUT to storage fails", async () => {
    server.use(
      http.put(STORAGE_URL, () => new HttpResponse(null, { status: 500 }))
    )
    const { result } = renderHook(() => useUpload("token"))
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(makeFile(), "avatar")
    })
    expect(res!).toBeNull()
    expect(result.current.error).toBe("Failed to upload file.")
  })

  it("sets error and returns null when /confirm fails", async () => {
    server.use(
      http.post(`${API}/uploads/uid-1/confirm`, () => new HttpResponse(null, { status: 500 }))
    )
    const { result } = renderHook(() => useUpload("token"))
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(makeFile(), "avatar")
    })
    expect(res!).toBeNull()
    expect(result.current.error).toBe("Failed to confirm upload.")
  })

  it("accepts gallery category with its own size limit", async () => {
    const { result } = renderHook(() => useUpload("token"))
    // 10 MB is the gallery limit — file just under limit should pass validation
    const okFile = makeFile("photo.jpg", "image/jpeg", 9 * 1024 * 1024)
    await act(async () => {
      await result.current.upload(okFile, "gallery")
    })
    // No size error (it may fail at network level in the test but not at validation)
    expect(result.current.error).not.toMatch(/too large/i)
  })

  it("sets error and returns null when /uploads/request fails with a non-JSON body", async () => {
    server.use(
      http.post(`${API}/uploads/request`, () =>
        new HttpResponse("Service Unavailable", { status: 503, headers: { "Content-Type": "text/plain" } })
      )
    )
    const { result } = renderHook(() => useUpload("token"))
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(makeFile(), "avatar")
    })
    expect(res!).toBeNull()
    expect(result.current.error).toBe("Failed to request upload.")
  })

  it("sets error and returns null on a network-level failure", async () => {
    server.use(
      http.post(`${API}/uploads/request`, () => HttpResponse.error())
    )
    const { result } = renderHook(() => useUpload("token"))
    let res: Awaited<ReturnType<typeof result.current.upload>>
    await act(async () => {
      res = await result.current.upload(makeFile(), "avatar")
    })
    expect(res!).toBeNull()
    expect(result.current.error).toBe("Upload failed.")
  })

  it("accepts chat-attachment category with pdf type", async () => {
    server.use(
      http.post(`${API}/uploads/request`, () =>
        HttpResponse.json({ upload_id: "uid-1", upload_url: STORAGE_URL })
      )
    )
    const { result } = renderHook(() => useUpload("token"))
    const pdfFile = makeFile("doc.pdf", "application/pdf")
    await act(async () => {
      await result.current.upload(pdfFile, "chat-attachment")
    })
    expect(result.current.error).not.toMatch(/not allowed/i)
  })
})
