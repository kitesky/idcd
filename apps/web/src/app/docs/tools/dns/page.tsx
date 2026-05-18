import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "tools/dns"

export const generateMetadata = () => docMetadata(SLUG)

export default async function ToolsDnsPage() {
  return renderDoc({ slug: SLUG })
}
