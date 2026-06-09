"use client"

import { useSession } from "next-auth/react"
import { useLocale, useTranslations } from "next-intl"
import { useCallback, useEffect, useState } from "react"
import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"
import { Link } from "@/i18n/navigation"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface Report {
  id: string
  reporter_id: string
  reported_user_id: string
  reported_email: string
  reported_name: string
  reported_username: string
  reported_avatar: string
  reason: string
  priority: string
  description: string
  status: string
  created_at: string
  reviewed_at?: string
  reviewed_by?: string
}

const STATUS_TABS = ["", "pending", "reviewed", "dismissed"] as const
const PRIORITY_TABS = ["", "critical", "high", "normal"] as const

const STATUS_BADGE: Record<string, string> = {
  pending: "bg-orange-900 text-orange-300",
  reviewed: "bg-green-900 text-green-300",
  dismissed: "bg-gray-800 text-gray-400",
}

const PRIORITY_BADGE: Record<string, string> = {
  critical: "bg-rose-900 text-rose-200 ring-1 ring-rose-700",
  high: "bg-amber-900 text-amber-200",
  normal: "bg-gray-800 text-gray-400",
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
  const t = useTranslations("admin")
  const reasonLabels: Record<string, string> = {
    harassment: t("reasonHarassment"),
    spam: t("reasonSpam"),
    inappropriate_content: t("reasonInappropriate"),
    fake_profile: t("reasonFakeProfile"),
    non_consensual_intimate_images: t("reasonNcii"),
    digital_gender_violence: t("reasonDigitalGenderViolence"),
    csam: t("reasonCsam"),
    other: t("reasonOther"),
  }
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
        setError(text.trim() || t("updateReportFailed"))
        return
      }
      onDone()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/80">
      <div className="w-full max-w-lg rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
        <h2 className="text-base font-semibold text-foreground">{t("reviewTitle")}</h2>

        <div className="rounded bg-gray-800 p-4 space-y-1 text-sm">
          <p className="text-gray-400">
            <span className="text-gray-200 font-medium">{t("reportedUserLabel")}:</span>{" "}
            {report.reported_username ? (
              <Link
                href={`/profile/${report.reported_username}`}
                target="_blank"
                rel="noopener noreferrer"
                className="text-brand-muted hover:underline"
              >
                {report.reported_name || report.reported_username} ↗
              </Link>
            ) : (
              <span>{report.reported_name || report.reported_email}</span>
            )}
          </p>
          <p className="text-gray-400">
            <span className="text-gray-200 font-medium">{t("reportReasonLabel")}:</span> {reasonLabels[report.reason] ?? report.reason}
          </p>
          {report.description && (
            <p className="text-gray-400">
              <span className="text-gray-200 font-medium">{t("reportDetailsLabel")}:</span> {report.description}
            </p>
          )}
        </div>

        {error && <p className="text-sm text-red-400">{error}</p>}

        <div>
          <label className="block text-xs text-gray-400 mb-2">{t("reviewResolution")}</label>
          <div className="flex gap-3">
            {(["reviewed", "dismissed"] as const).map((s) => (
              <button
                key={s}
                onClick={() => setNewStatus(s)}
                className={`cursor-pointer rounded px-3 py-1.5 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-brand-hover ${
                  newStatus === s
                    ? "bg-brand-primary text-foreground"
                    : "bg-gray-800 text-gray-300 hover:bg-gray-700"
                }`}
              >
                {s === "reviewed" ? t("reviewMarkReviewed") : t("reviewDismiss")}
              </button>
            ))}
          </div>
        </div>

        {newStatus === "reviewed" && (
          <div>
            <label className="block text-xs text-gray-400 mb-2">{t("reviewModAction")}</label>
            <div className="flex gap-3 mb-3">
              {(["", "suspend", "ban"] as const).map((a) => (
                <button
                  key={a}
                  onClick={() => setAction(a)}
                  className={`cursor-pointer rounded px-3 py-1.5 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-brand-hover ${
                    action === a
                      ? "bg-brand-primary text-foreground"
                      : "bg-gray-800 text-gray-300 hover:bg-gray-700"
                  }`}
                >
                  {a === "" ? t("reviewActionNone") : a === "suspend" ? t("reviewActionSuspend") : t("reviewActionBan")}
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
                  placeholder={t("reviewDurationPlaceholder")}
                />
                <input
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  className="w-full rounded bg-gray-800 px-3 py-2 text-sm text-gray-200 ring-1 ring-gray-700 focus:outline-none"
                  placeholder={t("reviewReasonPlaceholder")}
                />
              </div>
            )}
            {action === "ban" && (
              <input
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                className="w-full rounded bg-gray-800 px-3 py-2 text-sm text-gray-200 ring-1 ring-gray-700 focus:outline-none"
                placeholder={t("reviewBanReasonPlaceholder")}
              />
            )}
          </div>
        )}

        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>{t("cancel")}</Button>
          <Button variant="primary" loading={loading} onClick={submit}>{t("submitAction")}</Button>
        </div>
      </div>
    </div>
  )
}

export default function AdminReportsPage() {
  const { data: session } = useSession()
  const t = useTranslations("admin")
  const locale = useLocale()

  const statusLabels: Record<string, string> = {
    "": t("statusAll"),
    pending: t("statusPending"),
    reviewed: t("statusReviewed"),
    dismissed: t("statusDismissed"),
  }
  const priorityLabels: Record<string, string> = {
    "": t("priorityAny"),
    critical: t("priorityCritical"),
    high: t("priorityHigh"),
    normal: t("priorityNormal"),
  }
  const reasonLabels: Record<string, string> = {
    harassment: t("reasonHarassment"),
    spam: t("reasonSpam"),
    inappropriate_content: t("reasonInappropriate"),
    fake_profile: t("reasonFakeProfile"),
    non_consensual_intimate_images: t("reasonNcii"),
    digital_gender_violence: t("reasonDigitalGenderViolence"),
    csam: t("reasonCsam"),
    other: t("reasonOther"),
  }

  const [reports, setReports] = useState<Report[]>([])
  const [statusFilter, setStatusFilter] = useState<string>("pending")
  const [priorityFilter, setPriorityFilter] = useState<string>("")
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
      if (priorityFilter) params.set("priority", priorityFilter)
      const res = await fetch(`${API_URL}/reports?${params}`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (!res.ok) throw new Error(String(res.status))
      const data = await res.json()
      setReports(Array.isArray(data) ? data : [])
    } catch {
      setError(t("loadReportsFailed"))
    } finally {
      setLoading(false)
    }
  }, [session, statusFilter, priorityFilter, t])

  useEffect(() => { fetchReports() }, [fetchReports])

  return (
    <div className="p-4 sm:p-6 md:p-8">
      <h1 className="text-2xl font-bold text-foreground mb-6">{t("reportsTitle")}</h1>

      {/* Status tabs */}
      <div className="flex flex-wrap items-center gap-3 mb-6">
        <div className="flex gap-1 rounded-lg bg-gray-900 p-1 ring-1 ring-gray-800">
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
              {statusLabels[s]}
            </button>
          ))}
        </div>
        <select
          value={priorityFilter}
          onChange={(e) => setPriorityFilter(e.target.value)}
          aria-label={t("colPriority")}
          className="rounded-md border border-gray-700 bg-gray-900 px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:ring-1 focus:ring-brand-hover"
        >
          {PRIORITY_TABS.map((p) => (
            <option key={p} value={p}>
              {priorityLabels[p]}
            </option>
          ))}
        </select>
      </div>

      {error && <p className="mb-4 text-sm text-red-400">{error}</p>}

      {/* Table */}
      <div className="overflow-x-auto rounded-lg ring-1 ring-gray-800">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-900 text-xs uppercase tracking-wider text-gray-500">
            <tr>
              <th className="px-4 py-3">{t("colPriority")}</th>
              <th className="px-4 py-3">{t("colReportedUser")}</th>
              <th className="hidden md:table-cell px-4 py-3">{t("reportReasonLabel")}</th>
              <th className="px-4 py-3">{t("colStatus")}</th>
              <th className="hidden sm:table-cell px-4 py-3">{t("colDate")}</th>
              <th className="px-4 py-3 text-right">{t("actionsMenu")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i} className="bg-gray-950">
                  <td className="px-4 py-3"><Skeleton className="h-4 w-16" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-36" /></td>
                  <td className="hidden md:table-cell px-4 py-3"><Skeleton className="h-4 w-28" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-20" /></td>
                  <td className="hidden sm:table-cell px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="px-4 py-3 text-right"><Skeleton className="h-4 w-20 ml-auto" /></td>
                </tr>
              ))
            ) : reports.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-500 bg-gray-950">
                  {t("reportsNoFound")}
                </td>
              </tr>
            ) : (
              reports.map((r) => (
                <tr key={r.id} className="bg-gray-950 hover:bg-gray-900">
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${PRIORITY_BADGE[r.priority] ?? PRIORITY_BADGE.normal}`}>
                      {priorityLabels[r.priority] ?? r.priority}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    {r.reported_username ? (
                      <Link
                        href={`/profile/${r.reported_username}`}
                        className="text-gray-100 font-medium hover:text-brand-muted transition-colors"
                      >
                        {r.reported_name || r.reported_username}
                      </Link>
                    ) : (
                      <p className="text-gray-100 font-medium">{r.reported_name || "—"}</p>
                    )}
                    <p className="text-xs text-gray-500">{r.reported_email}</p>
                  </td>
                  <td className="hidden md:table-cell px-4 py-3 text-gray-300">
                    {reasonLabels[r.reason] ?? r.reason}
                    {r.description && (
                      <p className="text-xs text-gray-500 truncate max-w-xs">{r.description}</p>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[r.status] ?? "bg-gray-800 text-gray-400"}`}>
                      {statusLabels[r.status] ?? r.status}
                    </span>
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 text-gray-400">
                    {new Date(r.created_at).toLocaleDateString(locale)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {r.status === "pending" && (
                      <Button size="sm" variant="primary" onClick={() => setReviewTarget(r)}>
                        {t("reviewReport")}
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
