"use client"

import { useSession } from "next-auth/react"
import { useLocale, useTranslations } from "next-intl"
import { useEffect, useState } from "react"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface Stats {
  total_users: number
  active_users: number
  suspended_users: number
  banned_users: number
  deleted_users: number
  total_reports: number
  pending_reports: number
  total_rooms: number
}

function StatCard({ label, value, accent }: { label: string; value: number; accent?: string }) {
  const locale = useLocale()
  return (
    <div className="rounded-lg bg-gray-900 ring-1 ring-gray-800 p-5">
      <p className="text-xs font-medium uppercase tracking-wider text-gray-500">{label}</p>
      <p className={`mt-2 text-3xl font-bold ${accent ?? "text-foreground"}`}>{value.toLocaleString(locale)}</p>
    </div>
  )
}

function StatCardSkeleton() {
  return (
    <div className="rounded-lg bg-gray-900 ring-1 ring-gray-800 p-5 space-y-2">
      <Skeleton className="h-3 w-24" />
      <Skeleton className="h-8 w-16" />
    </div>
  )
}

export default function AdminDashboard() {
  const { data: session } = useSession()
  const t = useTranslations("admin")
  const [stats, setStats] = useState<Stats | null>(null)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!session?.accessToken) return
    fetch(`${API_URL}/admin/stats`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
      .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
      .then(setStats)
      .catch(() => setError(t("loadStatsFailed")))
  }, [session, t])

  return (
    <div className="p-4 sm:p-6 md:p-8">
      <h1 className="text-2xl font-bold text-foreground mb-8">{t("dashboardTitle")}</h1>

      {error && (
        <p className="mb-6 text-sm text-red-400">{error}</p>
      )}

      <section aria-label={t("dashboardUsersSection")}>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500">{t("dashboardUsersSection")}</h2>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 mb-8">
          {stats ? (
            <>
              <StatCard label={t("statTotal")} value={stats.total_users} />
              <StatCard label={t("statActive")} value={stats.active_users} accent="text-green-400" />
              <StatCard label={t("statSuspended")} value={stats.suspended_users} accent="text-yellow-400" />
              <StatCard label={t("statBanned")} value={stats.banned_users} accent="text-red-400" />
            </>
          ) : (
            Array.from({ length: 4 }).map((_, i) => <StatCardSkeleton key={i} />)
          )}
        </div>
      </section>

      <section aria-label={t("dashboardReportsSection")}>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500">{t("dashboardReportsSection")}</h2>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
          {stats ? (
            <>
              <StatCard label={t("statTotalReports")} value={stats.total_reports} />
              <StatCard label={t("statPendingReports")} value={stats.pending_reports} accent="text-orange-400" />
              <StatCard label={t("statRooms")} value={stats.total_rooms} />
            </>
          ) : (
            Array.from({ length: 3 }).map((_, i) => <StatCardSkeleton key={i} />)
          )}
        </div>
      </section>
    </div>
  )
}
