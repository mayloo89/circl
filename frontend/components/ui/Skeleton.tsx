interface SkeletonProps {
  className?: string
}

/**
 * Primitive animated placeholder block. Compose these to build page-specific
 * loading states (e.g. RoomSkeleton, ProfileSkeleton).
 */
export default function Skeleton({ className = "" }: SkeletonProps) {
  return <span className={`block animate-pulse rounded bg-gray-800 ${className}`} />
}
