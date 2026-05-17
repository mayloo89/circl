import type { Metadata, Viewport } from "next"
import { Nunito, DM_Sans } from "next/font/google"
import { getLocale } from "next-intl/server"
import "./globals.css"

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000"

export const metadata: Metadata = {
  metadataBase: new URL(SITE_URL),
  title: {
    default: "Circl",
    template: "%s · Circl",
  },
  description: "Plataforma de contacto privada con chat seguro.",
  applicationName: "Circl",
  manifest: "/manifest.webmanifest",
  icons: {
    icon: [
      { url: "/branding/favicon.ico", sizes: "any" },
      { url: "/branding/favicon-16.png", type: "image/png", sizes: "16x16" },
      { url: "/branding/favicon-32.png", type: "image/png", sizes: "32x32" },
      { url: "/branding/icon-192.png", type: "image/png", sizes: "192x192" },
      { url: "/branding/icon-512.png", type: "image/png", sizes: "512x512" },
    ],
    apple: [
      { url: "/branding/apple-touch-icon.png", sizes: "180x180" },
    ],
  },
  openGraph: {
    type: "website",
    siteName: "Circl",
    title: "Circl",
    description: "Plataforma de contacto privada con chat seguro.",
    images: [
      { url: "/branding/og-image.png", width: 1200, height: 630, alt: "Circl" },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "Circl",
    description: "Plataforma de contacto privada con chat seguro.",
    images: ["/branding/twitter-card.png"],
  },
}

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: "#0A0A0F",
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
