/**
 * Controllable stand-ins for WebSocket and EventSource. Tests drive the
 * connection lifecycle explicitly via open() / message() / error() / close().
 * Install with vi.stubGlobal("WebSocket", FakeWebSocket) and reset() in
 * beforeEach so instances don't leak between tests.
 */

export class FakeWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  static instances: FakeWebSocket[] = []
  static reset() {
    FakeWebSocket.instances = []
  }

  url: string
  readyState = FakeWebSocket.CONNECTING
  sent: string[] = []
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    if (this.readyState === FakeWebSocket.CLOSED) return
    this.readyState = FakeWebSocket.CLOSED
    this.onclose?.()
  }

  // Test drivers
  open() {
    this.readyState = FakeWebSocket.OPEN
    this.onopen?.()
  }

  message(frame: unknown) {
    this.onmessage?.({ data: typeof frame === "string" ? frame : JSON.stringify(frame) })
  }

  error() {
    this.onerror?.()
  }
}

export class FakeEventSource {
  static instances: FakeEventSource[] = []
  static reset() {
    FakeEventSource.instances = []
  }

  url: string
  closed = false
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null

  constructor(url: string) {
    this.url = url
    FakeEventSource.instances.push(this)
  }

  close() {
    this.closed = true
  }

  // Test drivers
  open() {
    this.onopen?.()
  }

  message(event: unknown) {
    this.onmessage?.({ data: typeof event === "string" ? event : JSON.stringify(event) })
  }

  error() {
    this.onerror?.()
  }
}
