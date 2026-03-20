import { API_ENDPOINTS } from "~/constants"

import type { ImportPreview, ImportResult } from "~/types"

export const importService = {
  async preview(source: string, file: File): Promise<ImportPreview> {
    const formData = new FormData()
    formData.append("file", file)

    const response = await fetch(
      `${API_ENDPOINTS.IMPORT.PREVIEW}?source=${encodeURIComponent(source)}`,
      {
        method: "POST",
        body: formData,
      }
    )

    if (!response.ok) {
      const data = await response.json()
      throw new Error(data.error || "Failed to preview import")
    }

    return response.json()
  },

  async importFlights(
    source: string,
    file: File,
    enrich: boolean
  ): Promise<ImportResult> {
    const formData = new FormData()
    formData.append("file", file)

    const params = new URLSearchParams({
      source,
      enrich: String(enrich),
    })

    const response = await fetch(`${API_ENDPOINTS.IMPORT.IMPORT}?${params}`, {
      method: "POST",
      body: formData,
    })

    if (!response.ok) {
      const data = await response.json().catch(() => ({
        error: `HTTP ${response.status}: ${response.statusText}`,
      }))
      throw new Error(data.error || "Failed to import flights")
    }

    const text = await response.text()
    if (!text) {
      throw new Error("Server returned an empty response")
    }

    return JSON.parse(text) as ImportResult
  },
}
