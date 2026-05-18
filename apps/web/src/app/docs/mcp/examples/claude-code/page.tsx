import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "mcp/examples/claude-code"

export const generateMetadata = () => docMetadata(SLUG)

export default async function McpClaudeCodePage() {
  return renderDoc({ slug: SLUG })
}
