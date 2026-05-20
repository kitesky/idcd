import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "tools/ssl"

export const generateMetadata = () => docMetadata(SLUG)

export default async function ToolsSslPage() {
  return renderDoc({ slug: SLUG })
}
