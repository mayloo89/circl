import { setRequestLocale, getTranslations } from "next-intl/server"

import LegalPage from "@/components/LegalPage"
import EsContent from "@/content/legal/safety.es.mdx"
import EnContent from "@/content/legal/safety.en.mdx"
import PtContent from "@/content/legal/safety.pt.mdx"

const lastUpdated = "2026-05-10"

export default async function SafetyPage({
  params,
}: {
  params: Promise<{ locale: string }>
}) {
  const { locale } = await params
  setRequestLocale(locale)

  const t = await getTranslations({ locale, namespace: "legal.safety" })
  const Content = locale === "es" ? EsContent : locale === "pt" ? PtContent : EnContent

  return (
    <LegalPage title={t("title")} lastUpdated={lastUpdated} draft={process.env.LEGAL_DRAFT === "true"}>
      <Content />
    </LegalPage>
  )
}
