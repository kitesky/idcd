import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "mcp/examples/python"

export const generateMetadata = () => docMetadata(SLUG)

export default async function McpPythonPage() {
  return renderDoc({ slug: SLUG })
}
