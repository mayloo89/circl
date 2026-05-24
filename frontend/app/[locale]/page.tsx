import HomeHero from "@/components/home/HomeHero"
import ProfileCompletenessBanner from "@/components/home/ProfileCompletenessBanner"
import PendingRequestsWidget from "@/components/home/PendingRequestsWidget"
import NearbyProfilesWidget from "@/components/home/NearbyProfilesWidget"
import RecentConversationsWidget from "@/components/home/RecentConversationsWidget"

export default function Home() {
  return (
    <>
      <HomeHero />
      <div className="mx-auto max-w-2xl lg:px-4 lg:pb-6">
        <div className="space-y-5 px-4 pb-8 lg:px-0">
          <PendingRequestsWidget />
          <NearbyProfilesWidget />
          <RecentConversationsWidget />
          <ProfileCompletenessBanner />
        </div>
      </div>
    </>
  )
}
