"use client"

import { useState } from "react"
import { Share2, Check } from "lucide-react"
import { Button } from "@/components/ui"

/**
 * Copy-current-URL button used on the public report pages (/r/[id] and the
 * legacy /report/[id]). Lives in its own client component because the
 * surrounding page is a server component.
 */
export default function ShareLinkButton() {
  const [copied, setCopied] = useState(false)

  const handleShare = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // fallback: ignored — modern browsers all support clipboard from a click handler
    }
  }

  return (
    <Button variant="outline" className="w-full justify-start" onClick={handleShare}>
      {copied ? (
        <Check className="mr-2 h-4 w-4 text-green-500" />
      ) : (
        <Share2 className="mr-2 h-4 w-4" />
      )}
      {copied ? "已复制报告链接" : "复制报告链接"}
    </Button>
  )
}
