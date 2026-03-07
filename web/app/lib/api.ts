export class ApiError extends Error {
  constructor(
    message: string,
    public statusCode: number
  ) {
    super(message)
    this.name = "ApiError"
  }
}

class ApiClient {
  private async request<T>(
    path: string,
    options: RequestInit = {}
  ): Promise<T> {
    const mergedOptions: RequestInit = {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...options.headers,
      },
    }

    const response = await fetch(path, mergedOptions)

    if (!response.ok) {
      const error = await response.json().catch(() => ({
        error: `HTTP ${response.status}: ${response.statusText}`,
      }))

      throw new ApiError(error.error || "Request failed", response.status)
    }

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

  async delete<T = unknown>(path: string): Promise<T> {
    return this.request<T>(path, { method: "DELETE" })
  }
}

export const api = new ApiClient()
