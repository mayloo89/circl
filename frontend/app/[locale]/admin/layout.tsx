import { redirect } from "next/navigation"
import { auth } from "@/lib/auth"
import { Link } from "@/i18n/navigation"

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

  return (
    <div className="flex min-h-screen bg-gray-950">
      {/* Sidebar */}
      <aside className="w-56 shrink-0 border-r border-gray-800 bg-gray-900">
        <div className="px-5 py-4 border-b border-gray-800">
          <Link href="/" className="text-base font-bold text-white hover:text-gray-300">
            Circl
          </Link>
          <p className="mt-0.5 text-xs font-medium text-brand-muted uppercase tracking-wider">Admin</p>
        </div>
        <nav className="mt-2 px-2 py-2 flex flex-col gap-0.5">
          <SidebarLink href="/admin">Dashboard</SidebarLink>
          <SidebarLink href="/admin/users">Users</SidebarLink>
          <SidebarLink href="/admin/reports">Reports</SidebarLink>
          <SidebarLink href="/admin/channels">Channels</SidebarLink>
        </nav>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-auto">{children}</main>
    </div>
  )
}

function SidebarLink({
  href,
  children,
}: {
  href: string
  children: React.ReactNode
}) {
  return (
    <Link
      href={href}
      className="block rounded px-3 py-2 text-sm text-gray-300 hover:bg-gray-800 hover:text-white transition-colors"
    >
      {children}
    </Link>
  )
}
