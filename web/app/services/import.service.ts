import { api } from "~/lib/api"
import { API_ENDPOINTS } from "~/constants"

import type { ImportJob, ImportPreview } from "~/types"

function fileForm(file: File): FormData {
  const form = new FormData()
  form.append("file", file)
  return form
}

export const importService = {
  async preview(source: string, file: File): Promise<ImportPreview> {
    const params = new URLSearchParams({ source })
    return api.upload<ImportPreview>(
      `${API_ENDPOINTS.IMPORT.PREVIEW}?${params}`,
      fileForm(file)
    )
  },

  // Starts a background import and returns the new job.
  async importFlights(
    source: string,
    file: File,
    enrich: boolean
  ): Promise<ImportJob> {
    const params = new URLSearchParams({ source, enrich: String(enrich) })
    return api.upload<ImportJob>(
      `${API_ENDPOINTS.IMPORT.IMPORT}?${params}`,
      fileForm(file)
    )
  },

  // Returns the running or most recently finished import, or null if none.
  async status(): Promise<ImportJob | null> {
    return api.get<ImportJob | null>(API_ENDPOINTS.IMPORT.STATUS)
  },
}
