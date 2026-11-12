import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { api, ApiError, UNAUTHORIZED_EVENT } from "./api"

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

describe("ApiClient", () => {
  const fetchMock = vi.fn<typeof fetch>()
  let events: EventTarget

  beforeEach(() => {
    events = new EventTarget()
    vi.stubGlobal("fetch", fetchMock)
    vi.stubGlobal("window", events)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  it("returns parsed JSON and sends JSON bodies", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { ok: true }))

    await expect(api.post("/api/v1/x", { a: 1 })).resolves.toEqual({ ok: true })

    const [, init] = fetchMock.mock.calls[0]
    expect(init?.method).toBe("POST")
    expect(init?.body).toBe('{"a":1}')
    expect(init?.headers).toMatchObject({ "Content-Type": "application/json" })
  })

  it("throws ApiError with the server's message and status", async () => {
    fetchMock.mockResolvedValue(jsonResponse(409, { error: "already added" }))

    const error = await api.get("/api/v1/x").catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).message).toBe("already added")
    expect((error as ApiError).statusCode).toBe(409)
  })

  it("falls back to the HTTP status when the error is not JSON", async () => {
    fetchMock.mockResolvedValue(
      new Response("<html>bad gateway</html>", {
        status: 502,
        statusText: "Bad Gateway",
      })
    )

    await expect(api.get("/api/v1/x")).rejects.toThrow("HTTP 502: Bad Gateway")
  })

  it("announces an expired session on 401, and only then", async () => {
    const onUnauthorized = vi.fn()
    events.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)

    fetchMock.mockResolvedValue(jsonResponse(400, { error: "bad request" }))
    await api.get("/api/v1/x").catch(() => {})
    expect(onUnauthorized).not.toHaveBeenCalled()

    fetchMock.mockResolvedValue(jsonResponse(401, { error: "invalid token" }))
    await api.get("/api/v1/x").catch(() => {})
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })

  it("returns null for an empty 204 response", async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }))

    await expect(api.get("/api/v1/import/status")).resolves.toBeNull()
  })

  it("lets the browser set the content type for uploads", async () => {
    fetchMock.mockResolvedValue(jsonResponse(202, { state: "running" }))

    const form = new FormData()
    form.append("file", new Blob(["a,b"]), "flights.csv")
    await api.upload("/api/v1/import", form)

    const [, init] = fetchMock.mock.calls[0]
    expect(init?.body).toBe(form)
    // A JSON content type here would break the multipart boundary.
    expect(init?.headers).toBeUndefined()
  })

  it("sends a JSON body with DELETE when given one", async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { message: "deleted" }))

    await api.delete("/api/v1/auth/me", { password: "secret" })

    const [, init] = fetchMock.mock.calls[0]
    expect(init?.method).toBe("DELETE")
    expect(init?.body).toBe('{"password":"secret"}')
  })
})
