import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "tools/http"

export const generateMetadata = () => docMetadata(SLUG)

export default async function ToolsHttpPage() {
  return renderDoc({ slug: SLUG })
}
