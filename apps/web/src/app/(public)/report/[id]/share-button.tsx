"use client"

import { useState } from "react"
import { Button } from "@/components/ui"
import { Share2, Check } from "lucide-react"

export default function ShareButton() {
  const [copied, setCopied] = useState(false)

  const handleShare = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // fallback: select text
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
