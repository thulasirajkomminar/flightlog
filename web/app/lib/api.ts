export class ApiError extends Error {
  constructor(
    message: string,
    public statusCode: number
  ) {
    super(message)
    this.name = "ApiError"
  }
}

// Fired on any 401 response so the app can drop the signed-in user and send
// them back to the login page when their session has expired.
export const UNAUTHORIZED_EVENT = "flightlog:unauthorized"

class ApiClient {
  // Sends a request and returns the raw response, throwing ApiError for any
  // non-2xx status. JSON is the default body type; FormData sets its own.
  private async send(path: string, options: RequestInit): Promise<Response> {
    const headers =
      options.body instanceof FormData
        ? options.headers
        : { "Content-Type": "application/json", ...options.headers }

    const response = await fetch(path, { ...options, headers })

    if (response.status === 401) {
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({
        error: `HTTP ${response.status}: ${response.statusText}`,
      }))

      throw new ApiError(error.error || "Request failed", response.status)
    }

    return response
  }

  private async request<T>(path: string, options: RequestInit): Promise<T> {
    const response = await this.send(path, options)

    if (
      response.status === 204 ||
      response.headers.get("content-length") === "0"
    ) {
      return null as unknown as T
    }

    return await response.json()
  }

  async get<T = unknown>(path: string): Promise<T> {
    return this.request<T>(path, { method: "GET" })
  }

  async post<T = unknown>(path: string, data?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "POST",
      body: data ? JSON.stringify(data) : undefined,
    })
  }

  async put<T = unknown>(path: string, data?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "PUT",
      body: data ? JSON.stringify(data) : undefined,
    })
  }

  async delete<T = unknown>(path: string, data?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: "DELETE",
      body: data ? JSON.stringify(data) : undefined,
    })
  }

  async upload<T = unknown>(path: string, form: FormData): Promise<T> {
    return this.request<T>(path, { method: "POST", body: form })
  }

  async download(path: string): Promise<Blob> {
    const response = await this.send(path, { method: "GET" })
    return response.blob()
  }
}

export const api = new ApiClient()
