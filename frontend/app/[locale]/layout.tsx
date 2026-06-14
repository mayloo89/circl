import type { Metadata } from "next"
import { NextIntlClientProvider } from "next-intl"
import { getMessages } from "next-intl/server"
import { notFound } from "next/navigation"
import { routing } from "@/i18n/routing"
import { auth } from "@/lib/auth"
import ClientErrorReporter from "@/components/ClientErrorReporter"
import Providers from "./providers"

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000"

export async function generateMetadata(): Promise<Metadata> {
  return {
    metadataBase: new URL(SITE_URL),
    title: { default: "Circl", template: "%s · Circl" },
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
      apple: [{ url: "/branding/apple-touch-icon.png", sizes: "180x180" }],
    },
    openGraph: {
      type: "website",
      siteName: "Circl",
      title: "Circl",
      description: "Plataforma de contacto privada con chat seguro.",
      images: [{ url: "/branding/og-image.png", width: 1200, height: 630, alt: "Circl" }],
    },
    twitter: {
      card: "summary_large_image",
      title: "Circl",
      description: "Plataforma de contacto privada con chat seguro.",
      images: ["/branding/twitter-card.png"],
    },
  }
}

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode
  params: Promise<{ locale: string }>
}) {
  const { locale } = await params

  if (!(routing.locales as readonly string[]).includes(locale)) {
    notFound()
  }

  const [messages, session] = await Promise.all([getMessages(), auth()])

  return (
    <NextIntlClientProvider messages={messages}>
      <ClientErrorReporter />
      <Providers session={session}>{children}</Providers>
    </NextIntlClientProvider>
  )
}
