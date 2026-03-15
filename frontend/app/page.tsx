import { auth } from "@/lib/auth"
import Link from "next/link"
import { redirect } from "next/navigation"

import SignOutButton from "@/components/SignOutButton"

export default async function Home() {
  const session = await auth()

  if (!session?.user) {
    redirect("/login")
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-950">
      <div className="w-full max-w-md space-y-4 rounded-lg bg-gray-900 p-8 shadow-xl ring-1 ring-gray-800">
        <div>
          <h1 className="text-center text-3xl font-bold text-white">Welcome to Circl</h1>
          <p className="mt-2 text-center text-gray-400">
            Hello, {session.user.name || session.user.email}
          </p>
        </div>

        <Link
          href="/profile"
          className="block w-full rounded-md bg-indigo-600 px-4 py-2 text-center text-white hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900"
        >
          My Profile
        </Link>

        <Link
          href="/contacts"
          className="block w-full rounded-md bg-indigo-600 px-4 py-2 text-center text-white hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900"
        >
          Contacts
        </Link>

        <SignOutButton />
      </div>
    </div>
  )
}
