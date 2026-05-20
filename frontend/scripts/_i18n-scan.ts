/**
 * Shared scanner for i18n tooling — extracts `useTranslations(ns)` + `t('key')`
 * usages from TS/TSX source, with proper scope handling for same-named `t`
 * bindings in different functions (regex-based, not full AST — false negatives
 * on cross-function variable passing are acceptable).
 *
 * Consumed by `scripts/i18n-stub.ts` (Phase 5.4 #2) and `scripts/lint-i18n.ts`
 * Rule 7 (Phase 5.4 #3).
 */

import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs'
import { join } from 'node:path'

// Captures all three flavors used in this project:
//   1. `const t = useTranslations('home')`           — next-intl client hook
//   2. `const t = await getT('home', locale)`        — local server helper
//      (src/i18n/getT.ts; only 1st arg is the namespace)
//   3. `const t = await getTranslations('home')`     — next-intl server helper
// Each call binds a `t`-like variable to a namespace; the binder's exact name
// (`useTranslations` / `getT` / `getTranslations`) doesn't matter downstream.
export const USE_TRANSLATIONS_BIND_RE =
  /(?:const|let|var)\s+(\w+)\s*=\s*(?:await\s+)?(?:useTranslations|getT|getTranslations)\((?:\s*['"]([^'"]+)['"][^)]*)?\)/g

// Also captures cross-function `t` passing where the receiving function annotates
// its parameter with a typed-prop signature:
//   `function X({ t }: { t: ReturnType<typeof useTranslations<"NS">> })`
//   `function X(arg: A, t: ReturnType<typeof useTranslations<"NS">>)`
// The named generic argument (`"NS"`) makes the namespace recoverable. Helpers
// without an explicit generic argument cannot be attributed and stay invisible
// to the scanner (false negatives are acceptable per module docs).
export const USE_TRANSLATIONS_TYPED_PARAM_RE =
  /\b(\w+)\s*:\s*ReturnType<typeof\s+(?:useTranslations|getT|getTranslations)<['"]([^'"]+)['"]>>/g

export const FILE_EXT_RE = /\.(ts|tsx)$/

const SKIP_DIRS = new Set(['node_modules', '.next', '__tests__'])
const SKIP_SUFFIX_RE = /\.(test|d)\.(ts|tsx)$/

export function walkSourceFiles(dir: string, out: string[]): void {
  if (!existsSync(dir)) return
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    const st = statSync(full)
    if (st.isDirectory()) {
      if (SKIP_DIRS.has(name)) continue
      walkSourceFiles(full, out)
    } else if (FILE_EXT_RE.test(name) && !SKIP_SUFFIX_RE.test(name)) {
      out.push(full)
    }
  }
}

/**
 * Scan a file's text for used i18n keys. Mutates `used`:
 *   used: Map<topNamespace, Set<fullKeyPathInsideThatJsonFile>>
 *
 * Same-named `t` bindings in sequential code blocks are attributed correctly
 * by sorting events by source position and walking through them.
 */
export function scanFile(content: string, used: Map<string, Set<string>>): void {
  type Event =
    | { type: 'bind'; pos: number; varName: string; nsPath: string[] }
    | { type: 'call'; pos: number; varName: string; key: string }
  const events: Event[] = []
  const candidateNames = new Set<string>()

  for (const m of content.matchAll(USE_TRANSLATIONS_BIND_RE)) {
    const varName = m[1]!
    const nsRaw = m[2]
    events.push({
      type: 'bind',
      pos: m.index ?? 0,
      varName,
      nsPath: nsRaw ? nsRaw.split('.') : [],
    })
    candidateNames.add(varName)
  }

  for (const m of content.matchAll(USE_TRANSLATIONS_TYPED_PARAM_RE)) {
    const varName = m[1]!
    const nsRaw = m[2]!
    events.push({
      type: 'bind',
      pos: m.index ?? 0,
      varName,
      nsPath: nsRaw.split('.'),
    })
    candidateNames.add(varName)
  }

  for (const name of candidateNames) {
    const callRe = new RegExp(`\\b${name}\\(\\s*(['"\`])([^'"\`$]+?)\\1`, 'g')
    for (const m of content.matchAll(callRe)) {
      events.push({ type: 'call', pos: m.index ?? 0, varName: name, key: m[2]! })
    }
  }

  events.sort((a, b) => a.pos - b.pos)

  const current = new Map<string, string[]>()
  for (const ev of events) {
    if (ev.type === 'bind') {
      current.set(ev.varName, ev.nsPath)
      continue
    }
    const nsPath = current.get(ev.varName)
    if (!nsPath) continue
    const fullPath = [...nsPath, ...ev.key.split('.')]
    if (fullPath.length < 2) continue
    const topNs = fullPath[0]!
    const innerKey = fullPath.slice(1).join('.')
    if (!used.has(topNs)) used.set(topNs, new Set())
    used.get(topNs)!.add(innerKey)
  }
}

/**
 * Used by Rule 7 / dead-key detection: a dynamic call like
 * `t(\`type.${tool.type}.label\`)` matches every `type.X.label` key, not
 * just the literal "type" or "label". We capture these patterns as regex so
 * callers can ask "is `type.http.label` covered by any dynamic match?".
 */
export interface DynamicPattern {
  topNs: string
  regex: RegExp
}

export interface ScanResult {
  used: Map<string, Set<string>>
  /** namespace → dynamic patterns whose key path contains `${...}` */
  dynamic: DynamicPattern[]
  filesScanned: number
}

const TEMPLATE_INTERP_RE = /\$\{[^}]+\}/g

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Compile a template-literal key path into a regex string.
 *   `type.${tool.type}.label` → `type\\.[^.]+\\.label`
 *
 * `${...}` segments are treated as "any single path component" (no dots).
 */
function templateToRegex(tpl: string): string {
  const parts: string[] = []
  let lastIdx = 0
  for (const m of tpl.matchAll(TEMPLATE_INTERP_RE)) {
    parts.push(escapeRegex(tpl.slice(lastIdx, m.index ?? 0)))
    parts.push('[^.]+')
    lastIdx = (m.index ?? 0) + m[0].length
  }
  parts.push(escapeRegex(tpl.slice(lastIdx)))
  return parts.join('')
}

/**
 * Scan template-literal calls with `${}` interpolation, plus `t.raw(...)` calls
 * (which return whole sub-trees, so they cover every descendant key). Same
 * scope rules as `scanFile`: events are sorted by source position and binding
 * state is tracked. Adds dynamic regex patterns to `out`.
 *
 * For `t.raw('foo.bar')` or `t.raw(\`foo.${x}\`)`, the resulting regex matches
 * the key itself **and** any descendant key — `t.raw` returns the entire JSON
 * sub-tree, so every key under that prefix is implicitly referenced.
 */
export function scanDynamicFile(
  content: string,
  out: DynamicPattern[],
): void {
  type Event =
    | { type: 'bind'; pos: number; varName: string; nsPath: string[] }
    | { type: 'tpl'; pos: number; varName: string; tpl: string; coversChildren: boolean }
  const events: Event[] = []
  const candidateNames = new Set<string>()

  for (const m of content.matchAll(USE_TRANSLATIONS_BIND_RE)) {
    events.push({
      type: 'bind',
      pos: m.index ?? 0,
      varName: m[1]!,
      nsPath: m[2] ? m[2].split('.') : [],
    })
    candidateNames.add(m[1]!)
  }

  for (const m of content.matchAll(USE_TRANSLATIONS_TYPED_PARAM_RE)) {
    events.push({
      type: 'bind',
      pos: m.index ?? 0,
      varName: m[1]!,
      nsPath: m[2]!.split('.'),
    })
    candidateNames.add(m[1]!)
  }

  // Index identifier → template literal value for the
  //   `const key = `${...}.foo`; t(key)`
  // indirection pattern. Risk is low because we only honour the binding when the
  // *same* identifier shows up inside a tracked `t(...)` later; unrelated tpl
  // literals (URLs, SQL, etc.) never collide with that.
  const tplLiteralBinds = new Map<string, string>()
  const tplBindRe =
    /(?:const|let|var)\s+(\w+)\s*=\s*`([^`]+)`/g
  for (const m of content.matchAll(tplBindRe)) {
    const tpl = m[2]!
    if (!tpl.includes('${')) continue
    tplLiteralBinds.set(m[1]!, tpl)
  }
  for (const name of candidateNames) {
    // Template-literal `t(`...`)` — only with `${...}` interpolation.
    const tplRe = new RegExp(`\\b${name}\\(\\s*\`([^\`]+)\``, 'g')
    for (const m of content.matchAll(tplRe)) {
      const tpl = m[1]!
      if (!tpl.includes('${')) continue
      events.push({ type: 'tpl', pos: m.index ?? 0, varName: name, tpl, coversChildren: false })
    }
    // Indirect call: `t(identifier)` where identifier was previously bound to a
    // template literal containing `${...}`.
    const identCallRe = new RegExp(`\\b${name}\\(\\s*(\\w+)\\s*[,)]`, 'g')
    for (const m of content.matchAll(identCallRe)) {
      const ident = m[1]!
      const tpl = tplLiteralBinds.get(ident)
      if (!tpl) continue
      events.push({ type: 'tpl', pos: m.index ?? 0, varName: name, tpl, coversChildren: false })
    }
    // `t.raw('lit')` or `t.raw(\`tpl\`)` — covers every descendant key.
    const rawRe = new RegExp(`\\b${name}\\.raw\\(\\s*(['"\`])([^'"\`]+?)\\1`, 'g')
    for (const m of content.matchAll(rawRe)) {
      events.push({
        type: 'tpl',
        pos: m.index ?? 0,
        varName: name,
        tpl: m[2]!,
        coversChildren: true,
      })
    }
  }

  events.sort((a, b) => a.pos - b.pos)
  const current = new Map<string, string[]>()
  for (const ev of events) {
    if (ev.type === 'bind') {
      current.set(ev.varName, ev.nsPath)
      continue
    }
    const nsPath = current.get(ev.varName)
    if (!nsPath) continue
    // useTranslations(NS) + t(`A.${x}.B`) → JSON path inside ${NS_TOP}.json is
    // `${NS_REST}.A.${...}.B`. Top-level namespace is fixed (nsPath[0] or a
    // static prefix of the template if there's no NS argument).
    const tplRegex = templateToRegex(ev.tpl)
    let topNs: string
    let innerPattern: string
    if (nsPath.length > 0) {
      topNs = nsPath[0]!
      const restNs = nsPath.slice(1).map(escapeRegex).join('\\.')
      innerPattern = restNs ? `${restNs}\\.${tplRegex}` : tplRegex
    } else {
      // useTranslations() 无 NS：template 第一段必须是静态前缀，否则跳过
      const firstDot = tplRegex.indexOf('\\.')
      if (firstDot < 0) continue
      const prefix = tplRegex.slice(0, firstDot)
      if (prefix.includes('[^.]+')) continue // 顶层就是动态，无法判断 NS
      topNs = prefix
      innerPattern = tplRegex.slice(firstDot + 2)
    }
    const childrenSuffix = ev.coversChildren ? '(?:\\.[^.]+)*' : ''
    out.push({ topNs, regex: new RegExp(`^${innerPattern}${childrenSuffix}$`) })
  }
}

export function scanProject(scanDirs: string[]): ScanResult {
  const files: string[] = []
  for (const dir of scanDirs) walkSourceFiles(dir, files)
  const used = new Map<string, Set<string>>()
  const dynamic: DynamicPattern[] = []
  for (const f of files) {
    try {
      const content = readFileSync(f, 'utf8')
      scanFile(content, used)
      scanDynamicFile(content, dynamic)
    } catch {
      // skip unreadable
    }
  }
  return { used, dynamic, filesScanned: files.length }
}

/**
 * Returns true iff `innerKey` is covered by any dynamic pattern for `topNs`.
 * Use this in dead-key checks before flagging a key as unused.
 */
export function isCoveredByDynamic(
  topNs: string,
  innerKey: string,
  dynamic: DynamicPattern[],
): boolean {
  for (const p of dynamic) {
    if (p.topNs !== topNs) continue
    if (p.regex.test(innerKey)) return true
  }
  return false
}
