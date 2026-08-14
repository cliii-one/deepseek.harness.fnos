interface ApiEnvelope<T> {
  code: number
  data?: T
  message?: string
}

async function request<T>(path: string, init?: RequestInit): Promise<T | null> {
  try {
    const res = await fetch(`api/${path}`, init)
    if (!res.ok) return null
    const body = (await res.json()) as ApiEnvelope<T>
    return body.code === 0 ? (body.data ?? null) : null
  } catch {
    return null
  }
}

const jsonInit = (method: string, body?: unknown): RequestInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: body === undefined ? undefined : JSON.stringify(body)
})

export const apiGet = <T>(path: string) => request<T>(path)
export const apiPost = <T>(path: string, body?: unknown) => request<T>(path, jsonInit('POST', body))
export const apiDelete = <T>(path: string) => request<T>(path, { method: 'DELETE' })
