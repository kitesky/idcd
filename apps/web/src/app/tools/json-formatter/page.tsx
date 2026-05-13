import type { Metadata } from "next"
import JsonFormatterClient from "./json-formatter-client"

export const metadata: Metadata = {
  title: 'JSON 格式化工具 - 在线 JSON 美化/压缩 | idcd',
  description: '免费在线 JSON 格式化、美化、压缩工具，支持语法检查，无需安装。',
}

export default function JsonFormatterPage() {
  return <JsonFormatterClient />
}