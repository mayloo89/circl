import type { Metadata, Viewport } from "next"

export const metadata: Metadata = {
  title: "Circl",
  description: "Private contact platform with secure chat",
}

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
}

// Minimal root layout — the [locale] layout provides <html> and <body>
// so we just pass through children here.
export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}
