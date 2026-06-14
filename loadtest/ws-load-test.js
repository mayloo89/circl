// WebSocket load test — concurrent guest connections to a public room.
//
// Exercises the real path the hub sees at peak: each virtual user creates a
// guest session, redeems a single-use WS ticket, opens a WebSocket to a public
// room, and periodically sends a message while receiving the room's broadcast
// fan-out. Each VU holds its socket open for its lifetime, so the number of
// active VUs equals the number of concurrent connections.
//
// Run (see loadtest/README.md for setup — the target backend must have the
// per-IP rate limits disabled, since all load comes from one IP):
//   k6 run loadtest/ws-load-test.js
//   BASE_URL=https://circl.example.com PEAK_VUS=500 k6 run loadtest/ws-load-test.js
import ws from "k6/ws"
import http from "k6/http"
import { check, sleep } from "k6"
import { Counter, Rate, Trend } from "k6/metrics"
import { randomString } from "https://jslib.k6.io/k6-utils/1.4.0/index.js"

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080"
const WS_URL = __ENV.WS_URL || BASE_URL.replace(/^http/, "ws")
// Origin must be in the backend's CORS_ALLOWED_ORIGINS or the upgrade is rejected.
const ORIGIN = __ENV.ORIGIN || "http://localhost:3000"
const ROOM_ID = __ENV.ROOM_ID || "" // empty → discovered from GET /guest/rooms

const PEAK_VUS = parseInt(__ENV.PEAK_VUS || "200", 10)
const RAMP = __ENV.RAMP || "30s"
const HOLD = __ENV.HOLD || "2m"
// How long each connection stays open, and how often it sends. The send gap
// stays above the guest message rate limit (default 6/min ≈ one per 10s).
const CONN_LIFETIME_S = parseInt(__ENV.CONN_LIFETIME_S || "60", 10)
const SEND_INTERVAL_S = parseInt(__ENV.SEND_INTERVAL_S || "15", 10)

const connectErrors = new Rate("ws_connect_errors")
const authErrors = new Rate("ws_auth_errors")
const msgsReceived = new Counter("ws_app_msgs_received")
const timeToOpen = new Trend("ws_time_to_open_ms", true)

export const options = {
  scenarios: {
    ramp_connections: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: RAMP, target: PEAK_VUS },
        { duration: HOLD, target: PEAK_VUS },
        { duration: RAMP, target: 0 },
      ],
      gracefulStop: "30s",
    },
  },
  thresholds: {
    ws_connect_errors: ["rate<0.05"], // <5% of VUs fail to open a socket
    ws_auth_errors: ["rate<0.02"], // session/ticket HTTP failures
    ws_connecting: ["p(95)<1500"], // handshake latency (k6 built-in)
    checks: ["rate>0.95"],
  },
}

// setup runs once: resolve a public room to target.
export function setup() {
  if (ROOM_ID) return { roomID: ROOM_ID }

  const res = http.get(`${BASE_URL}/guest/rooms`)
  check(res, { "GET /guest/rooms is 200": (r) => r.status === 200 })
  const rooms = res.json()
  if (!Array.isArray(rooms) || rooms.length === 0) {
    throw new Error(
      "no public rooms found — create one (admin) or pass ROOM_ID=<id>",
    )
  }
  return { roomID: rooms[0].id }
}

export default function (data) {
  const roomID = data.roomID
  const nickname = `lt_${__VU}_${randomString(6)}`

  // 1. Guest session.
  const sessionRes = http.post(
    `${BASE_URL}/guest/session`,
    JSON.stringify({ nickname, age_attestation: true, room_id: roomID }),
    { headers: { "Content-Type": "application/json" } },
  )
  const sessionOK = check(sessionRes, {
    "guest session created": (r) => r.status === 201,
  })
  if (!sessionOK) {
    // One sample per Rate per iteration — see the note below. Auth never
    // succeeded, so the connection wasn't attempted.
    authErrors.add(true)
    sleep(1)
    return
  }
  const sessionID = sessionRes.json("session_id")

  // 2. Single-use WS ticket.
  const ticketRes = http.post(
    `${BASE_URL}/guest/ws-ticket`,
    JSON.stringify({ session_id: sessionID }),
    { headers: { "Content-Type": "application/json" } },
  )
  const ticketOK = check(ticketRes, {
    "ws ticket issued": (r) => r.status === 200,
  })
  if (!ticketOK) {
    authErrors.add(true)
    sleep(1)
    return
  }
  const ticket = ticketRes.json("ticket")
  authErrors.add(false) // auth path completed for this iteration

  // 3. Hold a WebSocket open, send periodically, count broadcasts.
  const url = `${WS_URL}/chat/rooms/${roomID}/ws?ticket=${ticket}`
  const start = Date.now()
  let opened = false
  let socketErrored = false

  const res = ws.connect(url, { headers: { Origin: ORIGIN } }, function (socket) {
    socket.on("open", () => {
      opened = true
      timeToOpen.add(Date.now() - start)

      socket.setInterval(() => {
        socket.send(JSON.stringify({ type: "message", content: `hello from ${nickname}` }))
      }, SEND_INTERVAL_S * 1000)

      socket.setTimeout(() => socket.close(), CONN_LIFETIME_S * 1000)
    })

    socket.on("message", (raw) => {
      let frame
      try {
        frame = JSON.parse(raw)
      } catch {
        return
      }
      if (frame.event === "new_message") {
        msgsReceived.add(1)
      }
    })

    socket.on("error", () => {
      socketErrored = true
    })
  })

  // ws.connect returns once the socket closes. 101 = successful upgrade.
  const upgradeOK = check(res, { "ws upgrade is 101": (r) => r && r.status === 101 })
  // k6's Rate counts every add() as a sample, so add each metric exactly once
  // per iteration — otherwise the success/failure denominator is distorted and
  // the thresholds lie. This is the only connect-error sample for the iteration.
  connectErrors.add(!opened || socketErrored || !upgradeOK)
}
