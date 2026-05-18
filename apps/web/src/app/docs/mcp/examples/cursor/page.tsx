import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "mcp/examples/cursor"

export const generateMetadata = () => docMetadata(SLUG)

export default async function McpCursorPage() {
  return renderDoc({ slug: SLUG })
}
