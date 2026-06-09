import { setRequestLocale, getTranslations } from "next-intl/server"

import LegalPage from "@/components/LegalPage"
import EsContent from "@/content/legal/guidelines.es.mdx"
import EnContent from "@/content/legal/guidelines.en.mdx"
import PtContent from "@/content/legal/guidelines.pt.mdx"

const lastUpdated = "2026-05-10"

export default async function GuidelinesPage({
  params,
}: {
  params: Promise<{ locale: string }>
}) {
  const { locale } = await params
  setRequestLocale(locale)

  const t = await getTranslations({ locale, namespace: "legal.guidelines" })
  const Content = locale === "es" ? EsContent : locale === "pt" ? PtContent : EnContent

  return (
    <LegalPage title={t("title")} lastUpdated={lastUpdated} draft={process.env.LEGAL_DRAFT === "true"}>
      <Content />
    </LegalPage>
  )
}
