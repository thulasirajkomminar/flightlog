import { api } from "~/lib/api"
import { API_ENDPOINTS } from "~/constants"

import type { ImportPreview, ImportResult } from "~/types"

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

  async importFlights(
    source: string,
    file: File,
    enrich: boolean
  ): Promise<ImportResult> {
    const params = new URLSearchParams({ source, enrich: String(enrich) })
    return api.upload<ImportResult>(
      `${API_ENDPOINTS.IMPORT.IMPORT}?${params}`,
      fileForm(file)
    )
  },
}
