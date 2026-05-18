import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "mcp/authentication"

export const generateMetadata = () => docMetadata(SLUG)

export default async function McpAuthenticationPage() {
  return renderDoc({ slug: SLUG })
}
