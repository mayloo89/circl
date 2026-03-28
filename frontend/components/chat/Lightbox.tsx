import Modal from "@/components/ui/Modal"

interface LightboxProps {
  url: string
  type: "image" | "video"
  onClose: () => void
}

export default function Lightbox({ url, type, onClose }: LightboxProps) {
  return (
    <Modal open onClose={onClose}>
      <button
        aria-label="Close"
        className="absolute right-4 top-4 rounded-full p-2 text-white/70 hover:text-white"
        onClick={onClose}
      >
        <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
      {type === "video" ? (
        <video
          src={url}
          controls
          autoPlay
          className="max-h-[90vh] max-w-[90vw] rounded-lg"
          onClick={(e) => e.stopPropagation()}
        />
      ) : (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={url}
          alt=""
          className="max-h-[90vh] max-w-[90vw] rounded-lg object-contain"
          onClick={(e) => e.stopPropagation()}
        />
      )}
    </Modal>
  )
}
