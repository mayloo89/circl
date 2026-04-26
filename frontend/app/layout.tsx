import type { Metadata, Viewport } from "next"

export const metadata: Metadata = {
  title: "Circl",
  description: "Private contact platform with secure chat",
}

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html>
      <body>{children}</body>
    </html>
  )
}
