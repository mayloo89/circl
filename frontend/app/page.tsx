import { redirect } from "next/navigation"

// The middleware redirects / → /{defaultLocale}/ automatically.
// This page is a fallback for environments where the middleware isn't running.
export default function RootPage() {
  redirect("/es")
}
