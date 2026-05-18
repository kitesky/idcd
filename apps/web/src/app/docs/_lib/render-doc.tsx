import type { Metadata } from "next"
import { getLocale, getTranslations } from "next-intl/server"
import Link from "next/link"
import { Button } from "@/components/ui/button"
import { loadDocContent } from "@/lib/load-doc"

/**
 * 共享的文档页面渲染入口。
 *
 * 每个 `app/docs/.../page.tsx` 路由只剩两行：
 *
 *   export const generateMetadata = () => docMetadata("getting-started")
 *   export default () => renderDoc({ slug: "getting-started" })
 *
 * 实际内容来自 `apps/web/src/content/docs/{slug}/{locale}.mdx`，
 * 按 fallback 链选 locale（参见 `lib/load-doc.ts`）。
 */

export async function docMetadata(slug: string): Promise<Metadata> {
  const locale = await getLocale()
  try {
    const mod = await loadDocContent(slug, locale)
    return {
      title: mod.meta?.title,
      description: mod.meta?.description,
    }
  } catch {
    // 内容缺失时 metadata 留空，由 renderDoc 渲染 notFound 视图。
    return {}
  }
}

export async function renderDoc({ slug }: { slug: string }) {
  const locale = await getLocale()
  try {
    const mod = await loadDocContent(slug, locale)
    const Content = mod.default
    return (
      <article className="prose prose-zinc dark:prose-invert max-w-none">
        <Content />
      </article>
    )
  } catch {
    const t = await getTranslations("docs.notFound")
    return (
      <article className="prose prose-zinc dark:prose-invert max-w-none">
        <h1>{t("title")}</h1>
        <p>{t("description")}</p>
        <Button asChild variant="outline">
          <Link href="/docs">{t("backHome")}</Link>
        </Button>
      </article>
    )
  }
}
