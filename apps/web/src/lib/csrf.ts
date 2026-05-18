/**
 * CSRF protection utilities for client-side requests.
 * Implements double-submit cookie pattern with X-CSRF-Token header.
 */

import { API_CREDENTIALS_POLICY } from "@/lib/api"

const CSRF_COOKIE_NAME = 'csrf_token'
const CSRF_HEADER_NAME = 'X-CSRF-Token'

/**
 * Extracts CSRF token from document cookies.
 * @returns The CSRF token value, or null if not found.
 */
export function getCSRFToken(): string | null {
  if (typeof document === 'undefined') {
    return null // SSR context
  }

  const cookies = document.cookie.split(';')
  for (const cookie of cookies) {
    const parts = cookie.trim().split('=')
    const name = parts[0]
    const value = parts[1]
    if (name === CSRF_COOKIE_NAME) {
      return value ?? null
    }
  }
  return null
}

/**
 * Wrapper around fetch that automatically includes CSRF token in headers.
 * For mutating requests (POST, PUT, PATCH, DELETE), adds X-CSRF-Token header.
 * For GET requests, ensures CSRF cookie is set by making the request.
 *
 * @param url - The URL to fetch
 * @param options - Standard fetch options
 * @returns Promise<Response>
 */
export async function fetchWithCSRF(
  url: string,
  options?: RequestInit
): Promise<Response> {
  const method = options?.method?.toUpperCase() || 'GET'
  const headers = new Headers(options?.headers)

  // For mutating requests, include CSRF token
  if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
    const token = getCSRFToken()
    if (token) {
      headers.set(CSRF_HEADER_NAME, token)
    } else {
      console.warn('CSRF token not found - request may fail. Ensure GET request was made first.')
    }
  }

  // Merge headers back into options
  const requestOptions: RequestInit = {
    ...options,
    method,
    headers,
  }

  return fetch(url, requestOptions)
}

/**
 * Ensures CSRF token is initialized by making a GET request to the API.
 * Call this early in the application lifecycle (e.g., in _app.tsx or root layout)
 * to pre-populate the CSRF cookie before any mutating requests.
 *
 * @param endpoint - Optional endpoint to hit for token initialization (default: /v1/info/ip)
 */
export async function initCSRFToken(endpoint = '/v1/info/ip'): Promise<void> {
  if (typeof document === 'undefined') {
    return // Skip in SSR
  }

  const token = getCSRFToken()
  if (token) {
    return // Already initialized
  }

  try {
    // Make a lightweight GET request to trigger CSRF cookie generation.
    // Uses the shared same-domain credentials policy so a misconfigured
    // NEXT_PUBLIC_API_URL doesn't leak the bootstrap request to a third party.
    await fetch(endpoint, {
      method: 'GET',
      credentials: API_CREDENTIALS_POLICY,
    })
  } catch (error) {
    console.error('Failed to initialize CSRF token:', error)
  }
}
