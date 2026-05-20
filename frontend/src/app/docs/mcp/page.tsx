import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "mcp"

export const generateMetadata = () => docMetadata(SLUG)

export default async function McpPage() {
  return renderDoc({ slug: SLUG })
}
