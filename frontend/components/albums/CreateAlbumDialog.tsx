"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"

import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Modal from "@/components/ui/Modal"
import { albumsApi, type Album } from "@/lib/albums"

interface Props {
  open: boolean
  token: string
  onClose: () => void
  onCreated: (album: Album) => void
}

export default function CreateAlbumDialog({ open, token, onClose, onCreated }: Props) {
  const t = useTranslations("albums")
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError("")
    try {
      const album = await albumsApi.create(token, name, description)
      onCreated(album)
      setName("")
      setDescription("")
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed")
    } finally {
      setSubmitting(false)
    }
  }

  if (!open) return null

  return (
    <Modal open onClose={onClose}>
      <form
        onSubmit={submit}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-md rounded-xl bg-gray-900 p-6 shadow-2xl ring-1 ring-gray-700 space-y-4"
      >
        <h2 className="text-lg font-semibold text-white">{t("create")}</h2>
        <div className="space-y-1.5">
          <label htmlFor="album-name" className="block text-sm text-gray-300">
            {t("nameLabel")}
          </label>
          <Input
            id="album-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("namePlaceholder")}
            required
            maxLength={100}
            autoFocus
          />
        </div>
        <div className="space-y-1.5">
          <label htmlFor="album-desc" className="block text-sm text-gray-300">
            {t("descriptionLabel")}
          </label>
          <textarea
            id="album-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={t("descriptionPlaceholder")}
            maxLength={500}
            rows={3}
            className="w-full rounded border border-gray-700 bg-gray-950 px-3 py-2 text-sm text-white placeholder:text-gray-600 focus:border-brand-accent focus:outline-none focus:ring-1 focus:ring-brand-accent"
          />
        </div>
        {error && <p role="alert" className="text-sm text-red-400">{error}</p>}
        <div className="flex justify-end gap-3 pt-1">
          <Button type="button" variant="ghost" size="sm" onClick={onClose} disabled={submitting}>
            {t("cancel")}
          </Button>
          <Button type="submit" variant="primary" size="sm" loading={submitting} disabled={!name.trim()}>
            {submitting ? t("saving") : t("save")}
          </Button>
        </div>
      </form>
    </Modal>
  )
}
