"use client"

import Modal from "@/components/ui/Modal"
import Button from "@/components/ui/Button"

interface ReportDialogProps {
  open: boolean
  loading?: boolean
  error?: string
  onSubmit: (reason: string, description: string) => void
  onCancel: () => void
}

const REASONS = [
  { value: "harassment", label: "Harassment" },
  { value: "spam", label: "Spam" },
  { value: "inappropriate_content", label: "Inappropriate content" },
  { value: "fake_profile", label: "Fake profile" },
  { value: "other", label: "Other" },
]

export default function ReportDialog({
  open,
  loading = false,
  error,
  onSubmit,
  onCancel,
}: ReportDialogProps) {
  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const reason = formData.get("reason") as string
    const description = formData.get("description") as string
    onSubmit(reason, description)
  }

  return (
    <Modal open={open} onClose={onCancel}>
      <div
        className="w-full max-w-sm rounded-xl bg-gray-900 p-6 shadow-2xl ring-1 ring-gray-700"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold text-white">Report user</h2>
        <p className="mt-2 text-sm text-gray-400">
          Please select a reason for reporting this user.
        </p>
        <form onSubmit={handleSubmit} className="mt-4 space-y-4">
          <div>
            <label htmlFor="reason" className="block text-sm font-medium text-gray-300">
              Reason
            </label>
            <select
              id="reason"
              name="reason"
              required
              className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            >
              <option value="">Select a reason</option>
              {REASONS.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="description" className="block text-sm font-medium text-gray-300">
              Description (optional)
            </label>
            <textarea
              id="description"
              name="description"
              rows={3}
              className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              placeholder="Provide additional details..."
            />
          </div>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <div className="flex justify-end gap-3">
            <Button type="button" variant="ghost" size="sm" onClick={onCancel} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" variant="danger" size="sm" loading={loading}>
              Submit Report
            </Button>
          </div>
        </form>
      </div>
    </Modal>
  )
}