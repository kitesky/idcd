import { docMetadata, renderDoc } from "@/app/docs/_lib/render-doc"

const SLUG = "getting-started"

export const generateMetadata = () => docMetadata(SLUG)

export default async function GettingStartedPage() {
  return renderDoc({ slug: SLUG })
}
