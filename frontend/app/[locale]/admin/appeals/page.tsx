"use client"

import { useSession } from "next-auth/react"
import { useCallback, useEffect, useState } from "react"
import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface Appeal {
  id: string
  user_id: string
  user_email: string
  user_name: string
  suspension_id: string
  status: "open" | "submitted" | "approved" | "denied" | "expired"
  body: string
  expires_at: string
  submitted_at?: string
  resolved_at?: string
  resolved_by?: string
  resolution_note: string
  created_at: string
}

const STATUS_TABS = ["submitted", "open", "approved", "denied", ""] as const
const STATUS_LABELS: Record<string, string> = {
  "": "All",
  open: "Awaiting user",
  submitted: "Awaiting review",
  approved: "Approved",
  denied: "Denied",
  expired: "Expired",
}

const STATUS_BADGE: Record<string, string> = {
  open: "bg-gray-800 text-gray-300",
  submitted: "bg-amber-900 text-amber-200",
  approved: "bg-green-900 text-green-200",
  denied: "bg-rose-900 text-rose-200",
  expired: "bg-gray-800 text-gray-500",
}

function ReviewModal({
  appeal,
  token,
  onDone,
  onClose,
}: {
  appeal: Appeal
  token: string
  onDone: () => void
  onClose: () => void
}) {
  const [decision, setDecision] = useState<"approved" | "denied">("approved")
  const [note, setNote] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function submit() {
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/appeals/${appeal.id}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ status: decision, note }),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || "Failed to resolve appeal")
        return
      }
      onDone()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <div className="w-full max-w-xl rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
        <h2 className="text-base font-semibold text-foreground">Review appeal</h2>

        <div className="rounded bg-gray-800 p-4 space-y-2 text-sm">
          <p className="text-gray-400">
            <span className="text-gray-200 font-medium">User:</span> {appeal.user_name || appeal.user_email}
          </p>
          <p className="text-gray-400">
            <span className="text-gray-200 font-medium">Submitted:</span>{" "}
            {appeal.submitted_at ? new Date(appeal.submitted_at).toLocaleString() : "—"}
          </p>
          <div>
            <p className="text-gray-200 font-medium mb-1">Appeal:</p>
            <p className="whitespace-pre-wrap text-gray-300">{appeal.body || "(empty)"}</p>
          </div>
        </div>

        {error && <p className="text-sm text-red-400">{error}</p>}

        <div>
          <label className="block text-xs text-gray-400 mb-2">Decision</label>
          <div className="flex gap-3">
            {(["approved", "denied"] as const).map((d) => (
              <button
                key={d}
                onClick={() => setDecision(d)}
                className={`cursor-pointer rounded px-3 py-1.5 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-brand-hover ${
                  decision === d
                    ? d === "approved"
                      ? "bg-green-700 text-white"
                      : "bg-rose-800 text-white"
                    : "bg-gray-800 text-gray-300 hover:bg-gray-700"
                }`}
              >
                {d === "approved" ? "Approve (reactivate user)" : "Deny"}
              </button>
            ))}
          </div>
        </div>

        <div>
          <label htmlFor="note" className="block text-xs text-gray-400 mb-2">Note to user (optional)</label>
          <textarea
            id="note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={3}
            className="w-full rounded bg-gray-800 px-3 py-2 text-sm text-gray-200 ring-1 ring-gray-700 focus:outline-none focus:ring-brand-hover"
            placeholder="Briefly explain the decision; this is included in the resolution email."
          />
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="primary" loading={loading} onClick={submit}>Submit</Button>
        </div>
      </div>
    </div>
  )
}

export default function AdminAppealsPage() {
  const { data: session } = useSession()
  const [appeals, setAppeals] = useState<Appeal[]>([])
  const [statusFilter, setStatusFilter] = useState<string>("submitted")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [reviewTarget, setReviewTarget] = useState<Appeal | null>(null)

  const fetchAppeals = useCallback(async () => {
    if (!session?.accessToken) return
    setLoading(true)
    setError("")
    try {
      const params = new URLSearchParams()
      if (statusFilter) params.set("status", statusFilter)
      const res = await fetch(`${API_URL}/admin/appeals?${params}`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (!res.ok) throw new Error(String(res.status))
      const data = await res.json()
      setAppeals(Array.isArray(data) ? data : [])
    } catch {
      setError("Failed to load appeals")
    } finally {
      setLoading(false)
    }
  }, [session, statusFilter])

  useEffect(() => { fetchAppeals() }, [fetchAppeals])

  return (
    <div className="p-4 sm:p-6 md:p-8">
      <h1 className="text-2xl font-bold text-foreground mb-2">Appeals</h1>
      <p className="text-sm text-gray-400 mb-6">
        Suspended users can submit a written appeal via the link emailed to them. Approving an appeal reactivates the user.
      </p>

      <div className="flex gap-1 mb-6 rounded-lg bg-gray-900 p-1 w-fit ring-1 ring-gray-800">
        {STATUS_TABS.map((s) => (
          <button
            key={s}
            onClick={() => setStatusFilter(s)}
            className={`rounded px-4 py-1.5 text-sm font-medium transition-colors ${
              statusFilter === s
                ? "bg-brand-primary text-foreground"
                : "text-gray-400 hover:text-gray-200"
            }`}
          >
            {STATUS_LABELS[s]}
          </button>
        ))}
      </div>

      {error && <p className="mb-4 text-sm text-red-400">{error}</p>}

      <div className="overflow-x-auto rounded-lg ring-1 ring-gray-800">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-900 text-xs uppercase tracking-wider text-gray-500">
            <tr>
              <th className="px-4 py-3">User</th>
              <th className="px-4 py-3">Status</th>
              <th className="hidden sm:table-cell px-4 py-3">Submitted</th>
              <th className="hidden sm:table-cell px-4 py-3">Expires</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading ? (
              Array.from({ length: 4 }).map((_, i) => (
                <tr key={i} className="bg-gray-950">
                  <td className="px-4 py-3"><Skeleton className="h-4 w-36" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="hidden sm:table-cell px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="hidden sm:table-cell px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="px-4 py-3 text-right"><Skeleton className="h-4 w-20 ml-auto" /></td>
                </tr>
              ))
            ) : appeals.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-500 bg-gray-950">
                  No appeals found
                </td>
              </tr>
            ) : (
              appeals.map((a) => (
                <tr key={a.id} className="bg-gray-950 hover:bg-gray-900">
                  <td className="px-4 py-3">
                    <p className="text-gray-100 font-medium">{a.user_name || "—"}</p>
                    <p className="text-xs text-gray-500">{a.user_email}</p>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[a.status] ?? "bg-gray-800 text-gray-400"}`}>
                      {STATUS_LABELS[a.status] ?? a.status}
                    </span>
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 text-gray-400">
                    {a.submitted_at ? new Date(a.submitted_at).toLocaleDateString() : "—"}
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 text-gray-400">
                    {new Date(a.expires_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {a.status === "submitted" && (
                      <Button size="sm" variant="primary" onClick={() => setReviewTarget(a)}>
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
          appeal={reviewTarget}
          token={session.accessToken}
          onDone={() => { setReviewTarget(null); fetchAppeals() }}
          onClose={() => setReviewTarget(null)}
        />
      )}
    </div>
  )
}
