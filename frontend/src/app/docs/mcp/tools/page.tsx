import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "mcp/tools"

export const generateMetadata = () => docMetadata(SLUG)

export default async function McpToolsPage() {
  return renderDoc({ slug: SLUG })
}
