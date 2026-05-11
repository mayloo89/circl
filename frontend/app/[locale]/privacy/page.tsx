import { setRequestLocale, getTranslations } from "next-intl/server"

import LegalPage from "@/components/LegalPage"
import EsContent from "@/content/legal/privacy.es.mdx"
import EnContent from "@/content/legal/privacy.en.mdx"
import PtContent from "@/content/legal/privacy.pt.mdx"

const lastUpdated = "2026-05-10"

export default async function PrivacyPage({
  params,
}: {
  params: Promise<{ locale: string }>
}) {
  const { locale } = await params
  setRequestLocale(locale)

  const t = await getTranslations({ locale, namespace: "legal.privacy" })
  const Content = locale === "es" ? EsContent : locale === "pt" ? PtContent : EnContent

  return (
    <LegalPage title={t("title")} lastUpdated={lastUpdated} draft>
      <Content />
    </LegalPage>
  )
}
