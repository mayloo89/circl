"use client"

import { useSession } from "next-auth/react"
import { useCallback, useEffect, useState } from "react"
import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface Report {
  id: string
  reporter_id: string
  reported_user_id: string
  reported_email: string
  reported_name: string
  reported_avatar: string
  reason: string
  description: string
  status: string
  created_at: string
  reviewed_at?: string
  reviewed_by?: string
}

const STATUS_TABS = ["", "pending", "reviewed", "dismissed"] as const
const STATUS_LABELS: Record<string, string> = {
  "": "All",
  pending: "Pending",
  reviewed: "Reviewed",
  dismissed: "Dismissed",
}

const STATUS_BADGE: Record<string, string> = {
  pending: "bg-orange-900 text-orange-300",
  reviewed: "bg-green-900 text-green-300",
  dismissed: "bg-gray-800 text-gray-400",
}

const REASON_LABELS: Record<string, string> = {
  harassment: "Harassment",
  spam: "Spam",
  inappropriate_content: "Inappropriate content",
  fake_profile: "Fake profile",
  other: "Other",
}

// Modal for reviewing a report with optional moderation action
function ReviewModal({
  report,
  token,
  onDone,
  onClose,
}: {
  report: Report
  token: string
  onDone: () => void
  onClose: () => void
}) {
  const [newStatus, setNewStatus] = useState<"reviewed" | "dismissed">("reviewed")
  const [action, setAction] = useState<"" | "suspend" | "ban">("")
  const [reason, setReason] = useState("")
  const [days, setDays] = useState("7")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function submit() {
    setLoading(true)
    setError("")
    try {
      const body: Record<string, unknown> = { status: newStatus }
      if (action) {
        body.action = action
        body.reason = reason
        if (action === "suspend") body.duration_days = parseInt(days, 10) || 0
      }
      const res = await fetch(`${API_URL}/reports/${report.id}/status`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || "Failed to update report")
        return
      }
      onDone()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="w-full max-w-lg rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
        <h2 className="text-base font-semibold text-white">Review report</h2>

        <div className="rounded bg-gray-800 p-4 space-y-1 text-sm">
          <p className="text-gray-400">
            <span className="text-gray-200 font-medium">Reported user:</span> {report.reported_name || report.reported_email}
          </p>
          <p className="text-gray-400">
            <span className="text-gray-200 font-medium">Reason:</span> {REASON_LABELS[report.reason] ?? report.reason}
          </p>
          {report.description && (
            <p className="text-gray-400">
              <span className="text-gray-200 font-medium">Details:</span> {report.description}
            </p>
          )}
        </div>

        {error && <p className="text-sm text-red-400">{error}</p>}

        <div>
          <label className="block text-xs text-gray-400 mb-2">Resolution</label>
          <div className="flex gap-3">
            {(["reviewed", "dismissed"] as const).map((s) => (
              <button
                key={s}
                onClick={() => setNewStatus(s)}
                className={`rounded px-3 py-1.5 text-sm font-medium transition-colors ${
                  newStatus === s
                    ? "bg-indigo-600 text-white"
                    : "bg-gray-800 text-gray-300 hover:bg-gray-700"
                }`}
              >
                {s === "reviewed" ? "Mark reviewed" : "Dismiss"}
              </button>
            ))}
          </div>
        </div>

        {newStatus === "reviewed" && (
          <div>
            <label className="block text-xs text-gray-400 mb-2">Moderation action (optional)</label>
            <div className="flex gap-3 mb-3">
              {(["", "suspend", "ban"] as const).map((a) => (
                <button
                  key={a}
                  onClick={() => setAction(a)}
                  className={`rounded px-3 py-1.5 text-sm font-medium transition-colors ${
                    action === a
                      ? "bg-indigo-600 text-white"
                      : "bg-gray-800 text-gray-300 hover:bg-gray-700"
                  }`}
                >
                  {a === "" ? "None" : a === "suspend" ? "Suspend" : "Ban"}
                </button>
              ))}
            </div>
            {action === "suspend" && (
              <div className="space-y-2">
                <input
                  type="number"
                  min="0"
                  value={days}
                  onChange={(e) => setDays(e.target.value)}
                  className="w-full rounded bg-gray-800 px-3 py-2 text-sm text-gray-200 ring-1 ring-gray-700 focus:outline-none"
                  placeholder="Duration in days (0 = permanent)"
                />
                <input
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  className="w-full rounded bg-gray-800 px-3 py-2 text-sm text-gray-200 ring-1 ring-gray-700 focus:outline-none"
                  placeholder="Reason for moderation"
                />
              </div>
            )}
            {action === "ban" && (
              <input
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                className="w-full rounded bg-gray-800 px-3 py-2 text-sm text-gray-200 ring-1 ring-gray-700 focus:outline-none"
                placeholder="Reason for ban"
              />
            )}
          </div>
        )}

        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="primary" loading={loading} onClick={submit}>Submit</Button>
        </div>
      </div>
    </div>
  )
}

export default function AdminReportsPage() {
  const { data: session } = useSession()
  const [reports, setReports] = useState<Report[]>([])
  const [statusFilter, setStatusFilter] = useState<string>("pending")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [reviewTarget, setReviewTarget] = useState<Report | null>(null)

  const fetchReports = useCallback(async () => {
    if (!session?.accessToken) return
    setLoading(true)
    setError("")
    try {
      const params = new URLSearchParams()
      if (statusFilter) params.set("status", statusFilter)
      const res = await fetch(`${API_URL}/reports?${params}`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (!res.ok) throw new Error(String(res.status))
      const data = await res.json()
      setReports(Array.isArray(data) ? data : [])
    } catch {
      setError("Failed to load reports")
    } finally {
      setLoading(false)
    }
  }, [session, statusFilter])

  useEffect(() => { fetchReports() }, [fetchReports])

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold text-white mb-6">Reports</h1>

      {/* Status tabs */}
      <div className="flex gap-1 mb-6 rounded-lg bg-gray-900 p-1 w-fit ring-1 ring-gray-800">
        {STATUS_TABS.map((s) => (
          <button
            key={s}
            onClick={() => setStatusFilter(s)}
            className={`rounded px-4 py-1.5 text-sm font-medium transition-colors ${
              statusFilter === s
                ? "bg-indigo-600 text-white"
                : "text-gray-400 hover:text-gray-200"
            }`}
          >
            {STATUS_LABELS[s]}
          </button>
        ))}
      </div>

      {error && <p className="mb-4 text-sm text-red-400">{error}</p>}

      {/* Table */}
      <div className="overflow-x-auto rounded-lg ring-1 ring-gray-800">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-900 text-xs uppercase tracking-wider text-gray-500">
            <tr>
              <th className="px-4 py-3">Reported user</th>
              <th className="px-4 py-3">Reason</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Date</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i} className="bg-gray-950">
                  <td className="px-4 py-3"><Skeleton className="h-4 w-36" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-28" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-20" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="px-4 py-3 text-right"><Skeleton className="h-4 w-20 ml-auto" /></td>
                </tr>
              ))
            ) : reports.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-500 bg-gray-950">
                  No reports found
                </td>
              </tr>
            ) : (
              reports.map((r) => (
                <tr key={r.id} className="bg-gray-950 hover:bg-gray-900">
                  <td className="px-4 py-3">
                    <p className="text-gray-100 font-medium">{r.reported_name || "—"}</p>
                    <p className="text-xs text-gray-500">{r.reported_email}</p>
                  </td>
                  <td className="px-4 py-3 text-gray-300">
                    {REASON_LABELS[r.reason] ?? r.reason}
                    {r.description && (
                      <p className="text-xs text-gray-500 truncate max-w-xs">{r.description}</p>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[r.status] ?? "bg-gray-800 text-gray-400"}`}>
                      {r.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-400">
                    {new Date(r.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {r.status === "pending" && (
                      <Button size="sm" variant="primary" onClick={() => setReviewTarget(r)}>
                        Review
                      </Button>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {reviewTarget && session?.accessToken && (
        <ReviewModal
          report={reviewTarget}
          token={session.accessToken}
          onDone={() => { setReviewTarget(null); fetchReports() }}
          onClose={() => setReviewTarget(null)}
        />
      )}
    </div>
  )
}
