import { getTranslations } from "next-intl/server"
import ProfileCompletenessBanner from "@/components/home/ProfileCompletenessBanner"
import PendingRequestsWidget from "@/components/home/PendingRequestsWidget"
import NearbyProfilesWidget from "@/components/home/NearbyProfilesWidget"
import RecentConversationsWidget from "@/components/home/RecentConversationsWidget"

export default async function Home() {
  const t = await getTranslations("home")

  return (
    <div className="mx-auto max-w-2xl px-0 py-0 lg:px-4 lg:py-6">
      <ProfileCompletenessBanner />

      <div className="mt-4 space-y-6 px-4 pb-4 lg:px-0">
        <div>
          <h1 className="text-lg font-bold text-white">{t("welcome")}</h1>
        </div>

        <PendingRequestsWidget />
        <NearbyProfilesWidget />
        <RecentConversationsWidget />
      </div>
    </div>
  )
}
