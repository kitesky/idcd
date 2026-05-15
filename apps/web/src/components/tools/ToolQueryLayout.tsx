"use client"

/**
 * Shared layout for single-input info-query tool pages.
 * Handles the query/loading/error/result state pattern common to:
 * ASN, BGP, DMARC, ICP, IP, RDNS, SPF, SSL, WHOIS, MX, DKIM, etc.
 */

import { useState, useCallback, type KeyboardEvent, type ReactNode } from "react"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Button,
  Input,
  Label,
  Alert,
  AlertDescription,
} from "@/components/ui"

interface ToolQueryLayoutProps<T> {
  /** Page heading text */
  title: string
  /** Page subtitle / description */
  description: string
  /** Label for the primary query input */
  inputLabel: string
  /** Placeholder for the primary query input */
  inputPlaceholder: string
  /** Input id (for accessibility) */
  inputId: string
  /** Button label when idle */
  actionLabel?: string
  /** Button label while loading */
  loadingLabel?: string
  /** Card header title for the result section */
  resultCardTitle?: string
  /** Extra fields to render above the submit button (e.g. DKIM selector) */
  extraFields?: (loading: boolean) => ReactNode
  /** Build the query string from field values and invoke the API */
  onQuery: (query: string) => Promise<T>
  /** Render the result card content */
  renderResult: (result: T) => ReactNode
  /** Usage tips rendered at the bottom */
  tips: ReactNode
}

export function ToolQueryLayout<T>({
  title,
  description,
  inputLabel,
  inputPlaceholder,
  inputId,
  actionLabel = "查询",
  loadingLabel = "查询中...",
  onQuery,
  renderResult,
  tips,
  extraFields,
}: ToolQueryLayoutProps<T>) {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<T | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = useCallback(async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await onQuery(q)
      setResult(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "查询失败")
    } finally {
      setLoading(false)
    }
  }, [query, loading, onQuery])

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && !loading) handleSubmit()
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{title}</h1>
        <p className="text-muted-foreground mt-2">{description}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor={inputId}>{inputLabel}</Label>
            <div className="flex gap-2">
              <Input
                id={inputId}
                placeholder={inputPlaceholder}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={handleKeyDown}
                disabled={loading}
              />
              {!extraFields && (
                <Button
                  onClick={handleSubmit}
                  disabled={!query.trim() || loading}
                  className="min-w-[100px]"
                >
                  {loading ? loadingLabel : actionLabel}
                </Button>
              )}
            </div>
          </div>
          {extraFields && extraFields(loading)}
          {extraFields && (
            <Button
              onClick={handleSubmit}
              disabled={!query.trim() || loading}
              className="w-full"
            >
              {loading ? loadingLabel : actionLabel}
            </Button>
          )}
        </CardContent>
      </Card>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {result && renderResult(result)}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          {tips}
        </CardContent>
      </Card>
    </div>
  )
}
