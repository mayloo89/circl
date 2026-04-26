import type { Metadata, Viewport } from "next"
import { Nunito, DM_Sans } from "next/font/google"
import { getLocale } from "next-intl/server"
import "./globals.css"

export const metadata: Metadata = {
  title: "Circl",
  description: "Private contact platform with secure chat",
}

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
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
  const locale = await getLocale()
  return (
    <html lang={locale}>
      <body className={`${nunito.variable} ${dmSans.variable} antialiased`}>
        {children}
      </body>
    </html>
  )
}
