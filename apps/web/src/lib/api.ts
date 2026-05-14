export const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"

export async function apiRequest<T = any>(path: string, options?: RequestInit): Promise<T> {
  // Do not set a default Content-Type when the body is FormData — the browser
  // needs to set it automatically (including the multipart boundary parameter).
  const defaultHeaders: HeadersInit =
    options?.body instanceof FormData ? {} : { "Content-Type": "application/json" }

  const res = await fetch(API_BASE + path, {
    ...options,
    // credentials: "include" sends the HttpOnly access_token cookie automatically.
    // Tokens are no longer stored in localStorage.
    credentials: "include",
    headers: { ...defaultHeaders, ...options?.headers },
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

// Types
export interface Node {
  id: string
  name: string
  country_code: string
  region: string
  city: string
  asn: string
  isp: string
  tier: string
  status: string
  is_active: boolean
}

export interface ProbeParams {
  target: string
  node_ids?: string[]
  [key: string]: any
}

export interface ProbeResult {
  task_id: string
  status: string
  results?: Array<{
    node_id: string
    node_name: string
    success: boolean
    latency_ms?: number
    error?: string
    details?: any
  }>
}

// Nodes API
export async function getNodes(): Promise<{ data: Node[] }> {
  return apiRequest<{ data: Node[] }>("/v1/nodes")
}

// Probe API
export async function probeHttp(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/http", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function probePing(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/ping", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function probeTcp(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/tcping", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function probeDns(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/dns", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function probeTraceroute(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/traceroute", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function getProbeTask(taskId: string): Promise<ProbeResult> {
  return apiRequest<ProbeResult>(`/v1/probe/tasks/${taskId}`)
}

// Info API
export interface SSLInfo {
  domain: string
  issuer: string
  valid_from: string
  valid_to: string
  days_remaining: number
  is_valid: boolean
}

export interface WhoisInfo {
  domain: string
  registrar: string
  creation_date?: string
  expiration_date?: string
  registrant?: string
  name_servers?: string[]
}

export async function getSSLInfo(domain: string): Promise<SSLInfo> {
  return apiRequest<SSLInfo>(`/v1/info/ssl?q=${encodeURIComponent(domain)}`)
}

export async function getWhoisInfo(domain: string): Promise<WhoisInfo> {
  return apiRequest<WhoisInfo>(`/v1/info/whois?q=${encodeURIComponent(domain)}`)
}
