import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "mcp/quickstart"

export const generateMetadata = () => docMetadata(SLUG)

export default async function McpQuickstartPage() {
  return renderDoc({ slug: SLUG })
}
