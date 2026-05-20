import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "tools/ping"

export const generateMetadata = () => docMetadata(SLUG)

export default async function ToolsPingPage() {
  return renderDoc({ slug: SLUG })
}
