import { setRequestLocale, getTranslations } from "next-intl/server"

import LegalPage from "@/components/LegalPage"
import EsContent from "@/content/legal/terms.es.mdx"
import EnContent from "@/content/legal/terms.en.mdx"
import PtContent from "@/content/legal/terms.pt.mdx"

const lastUpdated = "2026-05-10"

export default async function TermsPage({
  params,
}: {
  params: Promise<{ locale: string }>
}) {
  const { locale } = await params
  setRequestLocale(locale)

  const t = await getTranslations({ locale, namespace: "legal.terms" })
  const Content = locale === "es" ? EsContent : locale === "pt" ? PtContent : EnContent

  return (
    <LegalPage title={t("title")} lastUpdated={lastUpdated} draft>
      <Content />
    </LegalPage>
  )
}
