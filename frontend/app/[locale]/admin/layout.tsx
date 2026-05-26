import { redirect } from "next/navigation"
import { auth } from "@/lib/auth"
import AdminShell from "@/components/admin/AdminShell"

export const metadata = { title: "Admin — Circl" }

export default async function AdminLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const session = await auth()
  if (session?.role !== "admin" && session?.role !== "super_admin") {
    redirect("/")
  }

  return <AdminShell>{children}</AdminShell>
}
