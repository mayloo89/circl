import type { ProfilePhoto } from "@/components/profile/PhotoGallery"
import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"

export type ContactStatus = "none" | "contact" | "sent" | "incoming" | "loading"

interface PublicProfile {
  user_id: string
  display_name: string
  bio: string
  avatar_url: string
  photos: ProfilePhoto[]
}

interface ProfileHeaderProps {
  profile: PublicProfile
  contactStatus: ContactStatus
  actionLoading: boolean
  isBlocked: boolean
  onAddContact: () => void
  onAccept: () => void
  onStartDM: () => void
  onBlock: () => void
  onUnblock: () => void
  onReport?: () => void
}

export default function ProfileHeader({
  profile,
  contactStatus,
  actionLoading,
  isBlocked,
  onAddContact,
  onAccept,
  onStartDM,
  onBlock,
  onUnblock,
  onReport,
}: ProfileHeaderProps) {
  return (
    <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
      <div className="flex flex-col items-center gap-4 text-center">
        <Avatar src={profile.avatar_url} name={profile.display_name || "?"} size="xl" />

        <div>
          <h2 className="text-xl font-bold text-white">{profile.display_name}</h2>
          {profile.bio && (
            <p className="mt-1 max-w-xs text-sm text-gray-400">{profile.bio}</p>
          )}
        </div>

        {isBlocked ? (
          <div className="flex flex-col items-center gap-2">
            <span className="rounded-full bg-red-950 px-4 py-1.5 text-xs text-red-400 ring-1 ring-red-900">
              You have blocked this user
            </span>
            <Button
              variant="ghost"
              size="sm"
              onClick={onUnblock}
              disabled={actionLoading}
            >
              Unblock
            </Button>
          </div>
        ) : (
          <>
            {contactStatus === "loading" && <Skeleton className="h-10 w-32 rounded-full" />}
            {contactStatus === "contact" && (
              <Button variant="accent" size="md" pill onClick={onStartDM} disabled={actionLoading}>
                Message
              </Button>
            )}
            {contactStatus === "sent" && (
              <span className="rounded-full bg-gray-800 px-5 py-2 text-sm text-gray-400 ring-1 ring-gray-700">
                Request sent
              </span>
            )}
            {contactStatus === "incoming" && (
              <Button variant="success" size="md" pill onClick={onAccept} disabled={actionLoading}>
                Accept request
              </Button>
            )}
            {contactStatus === "none" && (
              <Button
                variant="accent"
                size="md"
                pill
                onClick={onAddContact}
                disabled={actionLoading}
                className="flex items-center gap-2"
              >
                <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M18 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0ZM3 19.235v-.11a6.375 6.375 0 0 1 12.75 0v.109A12.318 12.318 0 0 1 9.374 21c-2.331 0-4.512-.645-6.374-1.766Z" />
                </svg>
                Add contact
              </Button>
            )}
            {contactStatus !== "loading" && (
              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={onBlock}
                  disabled={actionLoading}
                  className="text-xs text-gray-400 hover:text-red-400 disabled:cursor-not-allowed"
                >
                  Block user
                </button>
                {onReport && (
                  <button
                    type="button"
                    onClick={onReport}
                    disabled={actionLoading}
                    className="text-xs text-gray-400 hover:text-orange-400 disabled:cursor-not-allowed"
                  >
                    Report
                  </button>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
