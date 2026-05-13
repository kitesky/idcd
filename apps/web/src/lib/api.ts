export const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"

export async function apiRequest<T = any>(path: string, options?: RequestInit): Promise<T> {
  const token = typeof window !== "undefined" ? localStorage.getItem("access_token") : null
  const headers: HeadersInit = { "Content-Type": "application/json" }
  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  const res = await fetch(API_BASE + path, {
    ...options,
    headers: { ...headers, ...options?.headers }
  })

  if (!res.ok) {
    let errorMessage = "Request failed"
    try {
      const err = await res.json()
      errorMessage = err?.error?.message || err?.message || errorMessage
    } catch {
      // If response is not JSON, use status text
      errorMessage = res.statusText || errorMessage
    }
    throw new Error(errorMessage)
  }

  return res.json()
}
