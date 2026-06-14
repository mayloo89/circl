import { auth } from "@/lib/auth"
import Landing from "@/components/landing/Landing"
import HomeHero from "@/components/home/HomeHero"
import ProfileCompletenessBanner from "@/components/home/ProfileCompletenessBanner"
import PendingRequestsWidget from "@/components/home/PendingRequestsWidget"
import NearbyProfilesWidget from "@/components/home/NearbyProfilesWidget"
import RecentConversationsWidget from "@/components/home/RecentConversationsWidget"

export default async function Home() {
  const session = await auth()

  // Logged-out visitors get the public marketing landing; the authenticated
  // home (greeting + activity widgets) is only ever rendered for a session.
  if (!session) {
    return <Landing />
  }

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
