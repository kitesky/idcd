import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "tools/whois"

export const generateMetadata = () => docMetadata(SLUG)

export default async function ToolsWhoisPage() {
  return renderDoc({ slug: SLUG })
}
