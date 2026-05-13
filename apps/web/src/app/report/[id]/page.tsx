"use client"

import { useParams } from "next/navigation"
import { Card, CardContent, CardHeader, CardTitle, Button } from "@/components/ui"
import { Share2, FileDown, Clock } from "lucide-react"
import { useState } from "react"

export default function ReportPage() {
  const params = useParams()
  const reportId = params.id as string
  const [copied, setCopied] = useState(false)

  const handleShare = async () => {
    const url = window.location.href
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (err) {
      console.error("复制失败:", err)
    }
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">诊断报告</h1>
          <p className="text-muted-foreground mt-2">
            报告 ID: {reportId}
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="h-5 w-5" />
              报告生成中
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <p className="text-muted-foreground">
                您的诊断报告正在生成中，完整的诊断报告功能将在 S2 版本推出。
              </p>
              <p className="text-sm text-muted-foreground">
                当前版本提供实时诊断功能，您可以在诊断页面查看实时检测结果。
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>分享 & 导出</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Button
              variant="outline"
              className="w-full justify-start"
              onClick={handleShare}
            >
              <Share2 className="mr-2 h-4 w-4" />
              {copied ? "已复制链接" : "复制报告链接"}
            </Button>
            <Button
              variant="outline"
              className="w-full justify-start"
              disabled
            >
              <FileDown className="mr-2 h-4 w-4" />
              导出 PDF（S2 即将推出）
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>S2 版本功能预告</CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground space-y-2">
            <p>• <strong>完整报告</strong>：服务端渲染的详细诊断报告</p>
            <p>• <strong>历史记录</strong>：保存和查看历史诊断记录</p>
            <p>• <strong>PDF 导出</strong>：一键导出专业格式的诊断报告</p>
            <p>• <strong>趋势分析</strong>：跨时间段的性能趋势对比</p>
            <p>• <strong>智能建议</strong>：基于诊断结果的优化建议</p>
          </CardContent>
        </Card>

        <div className="flex justify-center">
          <Button
            variant="default"
            onClick={() => window.location.href = "/tools/diagnose"}
          >
            返回诊断工具
          </Button>
        </div>
      </div>
    </div>
  )
}
