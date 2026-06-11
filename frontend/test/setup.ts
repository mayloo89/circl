import "@testing-library/jest-dom"
import { beforeAll, afterEach, afterAll } from "vitest"
import { server } from "./msw-server"

// Node ≥ 22 defines an experimental `localStorage` global that is undefined
// unless --localstorage-file is passed, shadowing jsdom's implementation.
// Provide an in-memory Storage so code under test can use it normally.
if (globalThis.localStorage == null) {
  const store = new Map<string, string>()
  const impl: Storage = {
    getItem: (k) => store.get(k) ?? null,
    setItem: (k, v) => void store.set(k, String(v)),
    removeItem: (k) => void store.delete(k),
    clear: () => store.clear(),
    key: (i) => [...store.keys()][i] ?? null,
    get length() {
      return store.size
    },
  }
  Object.defineProperty(globalThis, "localStorage", { value: impl, configurable: true })
}

beforeAll(() => server.listen({ onUnhandledRequest: "warn" }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())
