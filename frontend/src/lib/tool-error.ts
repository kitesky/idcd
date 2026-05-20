/**
 * 工具函数（lib/tool-functions.ts）专用错误类型。
 *
 * 设计要点：
 * 1. tool-functions 是纯客户端工具函数（CIDR / JWT / IPv6 …），没有 React 上下文，
 *    无法直接调用 `useTranslations()`。
 * 2. 因此抛 `ToolError` 携带 i18n key + 参数；调用方在 React 层用 `translateToolError(err, t)` 翻译。
 * 3. `.message` 字段在抛出时同步填入 default locale（cn）翻译，保证以下两条不变量：
 *    a) 现有的 `e instanceof Error` / `e.message` 调用方继续按原样显示（CN）；
 *    b) 升级到 i18n 的调用方走 `translateToolError(err, t)` 拿到目标语言文案。
 *
 * 当 Phase 4d 之后的清理把所有 client 调用方都迁移到 `translateToolError` 之后，
 * `.message` 的 cn 默认值即可当 fallback。
 */
import cnDocs from '@/i18n/messages/cn/docs.json'

export type ToolErrorParams = Record<string, string | number>

const CN_ERRORS = cnDocs.toolFunctions.errors as Record<string, string>

function format(template: string, params?: ToolErrorParams): string {
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (_, key) =>
    Object.prototype.hasOwnProperty.call(params, key) ? String(params[key]) : `{${key}}`,
  )
}

function defaultMessage(key: string, params?: ToolErrorParams): string {
  const template = CN_ERRORS[key]
  if (!template) return key
  return format(template, params)
}

export class ToolError extends Error {
  readonly i18nKey: string
  readonly i18nParams?: ToolErrorParams

  constructor(key: string, params?: ToolErrorParams) {
    super(defaultMessage(key, params))
    this.name = 'ToolError'
    this.i18nKey = key
    this.i18nParams = params
    // 保留原型链（让 instanceof 工作于编译后的代码）。
    Object.setPrototypeOf(this, ToolError.prototype)
  }
}

/**
 * 在 React 组件里把任意 error 翻译成本地化文本。
 *
 * 调用方传 `useTranslations()`(全局) 或 `useTranslations('docs')` 等，t 函数会
 * 在内部用 ToolError 的 i18nKey (运行时拼出的字符串) 查表。运行时 key
 * next-intl 无法静态校验,helper 内部用 `as never` 收口。
 *
 * @example
 *   const t = useTranslations()
 *   try { ... } catch (e) { setError(translateToolError(e, t, 'fallback')) }
 *
 * @param err 被 catch 的 error 对象
 * @param t   next-intl 翻译函数 (任意 namespace 或全局)
 * @param fallback 当不是 ToolError 时的兜底文案，默认用 `err.message`
 */
// next-intl 4 收紧了 Translator 类型 — Translator<…, NS> 的 params 现在按 key 推断，
// 没有占位符的 key 会被推断成 `params: undefined`，导致 `t(key as never, params)`
// 的 params 类型不可赋值。这里用 unknown 收口，内部 cast 成宽松签名，调用方
// （`useTranslations(...)` 的返回值）可以无障碍传进来。
type LooseT = (key: string, params?: ToolErrorParams) => string

export function translateToolError(
  err: unknown,
  t: unknown,
  fallback?: string,
): string {
  if (err instanceof ToolError) {
    return (t as LooseT)(err.i18nKey, err.i18nParams)
  }
  if (err instanceof Error) return fallback ?? err.message
  return fallback ?? String(err)
}
