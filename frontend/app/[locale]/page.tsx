import HomeHero from "@/components/home/HomeHero"
import ProfileCompletenessBanner from "@/components/home/ProfileCompletenessBanner"
import PendingRequestsWidget from "@/components/home/PendingRequestsWidget"
import NearbyProfilesWidget from "@/components/home/NearbyProfilesWidget"
import RecentConversationsWidget from "@/components/home/RecentConversationsWidget"

export default function Home() {
  return (
    <div className="mx-auto max-w-2xl lg:px-4 lg:py-6">
      <HomeHero />
      <div className="space-y-4 px-4 pb-8 lg:px-0">
        <ProfileCompletenessBanner />
        <PendingRequestsWidget />
        <NearbyProfilesWidget />
        <RecentConversationsWidget />
      </div>
    </div>
  )
}
