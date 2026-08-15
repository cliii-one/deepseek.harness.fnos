/** API 契约：2xx 返回裸数据，4xx/5xx 返回 { message }，null 为网络失败 */
export type ApiResult<T> = { ok: true; data: T } | { ok: false; message: string }

async function request<T>(path: string, init?: RequestInit): Promise<ApiResult<T> | null> {
  try {
    const res = await fetch(`api/${path}`, init)
    const body = (await res.json().catch(() => null)) as Record<string, unknown> | T | null
    if (res.ok) {
      return { ok: true, data: body as T }
    }
    const message = (body as { message?: string } | null)?.message ?? `请求失败 (${res.status})`
    return { ok: false, message }
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
export const apiUpload = <T>(path: string, file: File) => {
  const fd = new FormData()
  fd.append('file', file)
  return request<T>(path, { method: 'POST', body: fd })
}
