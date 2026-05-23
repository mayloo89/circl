import type { Viewport } from "next"
import { Nunito, DM_Sans } from "next/font/google"
import { cookies } from "next/headers"
import { getLocale } from "next-intl/server"
import "./globals.css"

export const dynamic = "force-dynamic"

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: [
    { media: "(prefers-color-scheme: dark)",  color: "#0A1020" },
    { media: "(prefers-color-scheme: light)", color: "#F4F9FF" },
  ],
}

const nunito = Nunito({
  variable: "--font-nunito",
  subsets: ["latin"],
  display: "swap",
})

const dmSans = DM_Sans({
  variable: "--font-dm-sans",
  subsets: ["latin"],
  display: "swap",
})

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const [locale, cookieStore] = await Promise.all([getLocale(), cookies()])
  const theme = cookieStore.get("theme")?.value === "light" ? "" : "dark"
  return (
    <html lang={locale} className={theme} suppressHydrationWarning>
      <body className={`${nunito.variable} ${dmSans.variable} antialiased`}>
        {children}
      </body>
    </html>
  )
}
