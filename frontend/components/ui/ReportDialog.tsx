"use client"

import { useTranslations } from "next-intl"
import Modal from "@/components/ui/Modal"
import Button from "@/components/ui/Button"

interface ReportDialogProps {
  open: boolean
  loading?: boolean
  error?: string
  onSubmit: (reason: string, description: string) => void
  onCancel: () => void
}

const REASON_VALUES = [
  { value: "csam", key: "csam" },
  { value: "non_consensual_intimate_images", key: "nonConsensualIntimateImages" },
  { value: "digital_gender_violence", key: "digitalGenderViolence" },
  { value: "harassment", key: "harassment" },
  { value: "spam", key: "spam" },
  { value: "inappropriate_content", key: "inappropriateContent" },
  { value: "fake_profile", key: "fakeProfile" },
  { value: "other", key: "other" },
] as const

export default function ReportDialog({
  open,
  loading = false,
  error,
  onSubmit,
  onCancel,
}: ReportDialogProps) {
  const t = useTranslations("report")
  const tc = useTranslations("common")

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
        <h2 className="text-lg font-semibold text-white">{t("title")}</h2>
        <p className="mt-2 text-sm text-gray-400">
          {t("subtitle")}
        </p>
        <form onSubmit={handleSubmit} className="mt-4 space-y-4">
          <div>
            <label htmlFor="reason" className="block text-sm font-medium text-gray-300">
              {t("reasonLabel")}
            </label>
            <select
              id="reason"
              name="reason"
              required
              className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white shadow-sm focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"
            >
              <option value="">{t("reasonPlaceholder")}</option>
              {REASON_VALUES.map((r) => (
                <option key={r.value} value={r.value}>
                  {t(`reasons.${r.key}`)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="description" className="block text-sm font-medium text-gray-300">
              {t("descriptionLabel")}
            </label>
            <textarea
              id="description"
              name="description"
              rows={3}
              className="mt-1 block w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"
              placeholder={t("descriptionPlaceholder")}
            />
          </div>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <div className="flex justify-end gap-3">
            <Button type="button" variant="ghost" size="sm" onClick={onCancel} disabled={loading}>
              {tc("cancel")}
            </Button>
            <Button type="submit" variant="danger" size="sm" loading={loading}>
              {t("submit")}
            </Button>
          </div>
        </form>
      </div>
    </Modal>
  )
}
