import type { ComponentType } from 'react'
import { fallbackChain } from '@/i18n/registry'

export interface DocModule {
  default: ComponentType
  // 可选导出：每个 MDX 顶部可以 export const meta = { title, description }
  meta?: {
    title?: string
    description?: string
    keywords?: string[]
  }
}

/**
 * 按 fallback 链动态加载 MDX 文档。
 *
 * 约定文件路径：`apps/web/src/content/docs/{slug}/{locale}.mdx`
 *
 * 加新语言：只需添加 `${slug}/${newLocale}.mdx` 文件，零代码改动。
 *
 * @param slug 路径标识，如 `getting-started`、`mcp/quickstart`
 * @param locale 请求语言（cn / en / ja …）
 * @throws 当所有 fallback 都找不到内容时抛 Error
 */
export async function loadDocContent(
  slug: string,
  locale: string,
): Promise<DocModule> {
  for (const loc of fallbackChain(locale)) {
    try {
      // webpack/turbopack 静态分析需要字面前缀，所以模板字符串保持这种形式。
      const mod = (await import(`@/content/docs/${slug}/${loc}.mdx`)) as DocModule
      return mod
    } catch {
      // 当前 locale 没文件，继续尝试 fallback 链下一项。
    }
  }
  throw new Error(`[load-doc] No content found for slug="${slug}" locale="${locale}"`)
}

/**
 * 探测 slug 是否存在（任一 locale），通常用于 generateStaticParams 或 404 检测。
 */
export async function hasDocContent(slug: string, locale: string): Promise<boolean> {
  for (const loc of fallbackChain(locale)) {
    try {
      await import(`@/content/docs/${slug}/${loc}.mdx`)
      return true
    } catch {
      // try next
    }
  }
  return false
}
