import { notFound } from "next/navigation"

// Unknown paths inside a valid locale render the localized not-found page
// instead of Next's default 404, which has no locale context.
export default function CatchAllPage() {
  notFound()
}
