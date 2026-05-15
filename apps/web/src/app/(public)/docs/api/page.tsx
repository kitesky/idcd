import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { API_GROUPS, type HttpMethod } from "./api-reference-data"

export const metadata = {
  title: "API 参考 | idcd",
  description: "idcd 全球网络诊断 API 完整参考文档，包含拨测、IP查询、监控等接口",
}

const METHOD_COLORS: Record<HttpMethod, string> = {
  GET: "bg-blue-500/15 text-blue-400 border-blue-500/30",
  POST: "bg-green-500/15 text-green-400 border-green-500/30",
  PATCH: "bg-yellow-500/15 text-yellow-400 border-yellow-500/30",
  PUT: "bg-orange-500/15 text-orange-400 border-orange-500/30",
  DELETE: "bg-red-500/15 text-red-400 border-red-500/30",
}

function MethodBadge({ method }: { method: HttpMethod }) {
  return (
    <span
      className={`inline-flex items-center rounded border px-1.5 py-0.5 font-mono text-xs font-semibold ${METHOD_COLORS[method]}`}
    >
      {method}
    </span>
  )
}

export default function APIReferencePage() {
  return (
    <div className="w-full min-h-screen">
      {/* Page header */}
      <div className="border-b bg-background/95 py-8">
        <div className="w-full mx-auto max-w-screen-xl px-6">
          <h1 className="text-2xl font-bold tracking-tight">API 参考</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            idcd 全球网络诊断 API 完整参考文档。Base URL：{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
              https://api.idcd.com/v1
            </code>
          </p>
          <div className="mt-3 flex flex-wrap gap-3 text-xs text-muted-foreground">
            {[
              { color: "bg-blue-400", label: "GET" },
              { color: "bg-green-400", label: "POST" },
              { color: "bg-yellow-400", label: "PATCH" },
              { color: "bg-red-400", label: "DELETE" },
            ].map(({ color, label }) => (
              <span key={label} className="flex items-center gap-1.5">
                <span className={`inline-block h-2 w-2 rounded-full ${color}`} />
                {label}
              </span>
            ))}
          </div>
        </div>
      </div>

      {/* 3-col layout */}
      <div className="w-full mx-auto max-w-screen-xl flex">

        {/* ── Left nav ── */}
        <aside className="hidden lg:block w-52 shrink-0 border-r">
          <div className="sticky top-16 overflow-y-auto py-8 pr-4 pl-6" style={{ maxHeight: "calc(100vh - 4rem)" }}>
            <p className="mb-3 text-[11px] font-semibold uppercase tracking-widest text-muted-foreground/50">
              API 端点
            </p>
            <nav>
              <ul className="space-y-3">
                {API_GROUPS.map((group) => (
                  <li key={group.id}>
                    <a
                      href={`#group-${group.id}`}
                      className="block text-sm font-medium text-foreground/80 hover:text-foreground transition-colors"
                    >
                      {group.label}
                    </a>
                    <ul className="mt-1 ml-2 space-y-0.5 border-l border-border pl-3">
                      {group.endpoints.map((ep) => (
                        <li key={ep.id}>
                          <a
                            href={`#ep-${ep.id}`}
                            className="block py-0.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
                          >
                            {ep.summary}
                          </a>
                        </li>
                      ))}
                    </ul>
                  </li>
                ))}
              </ul>
            </nav>
          </div>
        </aside>

        {/* ── Main content ── */}
        <main className="min-w-0 flex-1 px-8 py-10 space-y-12">
            {/* Auth note */}
            <Card className="border-blue-500/30 bg-blue-500/5">
              <CardContent className="pt-4">
                <div className="text-sm">
                  <strong>鉴权</strong>：使用{" "}
                  <code className="rounded bg-muted px-1 font-mono text-xs">
                    Authorization: Bearer &lt;token&gt;
                  </code>{" "}
                  或{" "}
                  <code className="rounded bg-muted px-1 font-mono text-xs">
                    X-API-Key: &lt;api_key&gt;
                  </code>{" "}
                  。API Key 格式：{" "}
                  <code className="rounded bg-muted px-1 font-mono text-xs">idc_live_xxx</code>
                  。不需要鉴权的端点标记为{" "}
                  <Badge variant="secondary" className="text-xs">公开</Badge>。
                </div>
              </CardContent>
            </Card>

            {/* Endpoint groups */}
            {API_GROUPS.map((group) => (
              <section key={group.id} id={`group-${group.id}`} aria-labelledby={`group-heading-${group.id}`}>
                <div className="mb-6">
                  <h2 id={`group-heading-${group.id}`} className="text-2xl font-semibold">
                    {group.label}
                  </h2>
                  <p className="mt-1 text-sm text-muted-foreground">{group.description}</p>
                </div>

                <div className="space-y-4">
                  {group.endpoints.map((ep) => (
                    <Card key={ep.id} id={`ep-${ep.id}`}>
                      <CardHeader className="pb-3">
                        <CardTitle className="flex flex-wrap items-center gap-2 text-base">
                          <MethodBadge method={ep.method} />
                          <code className="font-mono text-sm font-normal text-foreground">
                            {ep.path}
                          </code>
                          <span className="text-sm font-normal text-muted-foreground">
                            — {ep.summary}
                          </span>
                          {ep.auth ? (
                            <Badge variant="outline" className="ml-auto text-xs">
                              需要鉴权
                            </Badge>
                          ) : (
                            <Badge variant="secondary" className="ml-auto text-xs">
                              公开
                            </Badge>
                          )}
                        </CardTitle>
                        <p className="text-sm text-muted-foreground">{ep.description}</p>
                      </CardHeader>

                      {((ep.parameters && ep.parameters.length > 0) || ep.responseExample) && (
                        <CardContent className="space-y-4">
                          {ep.parameters && ep.parameters.length > 0 && (
                            <div>
                              <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                                参数
                              </h4>
                              <Table>
                                <TableHeader>
                                  <TableRow>
                                    <TableHead className="w-32">名称</TableHead>
                                    <TableHead className="w-16">位置</TableHead>
                                    <TableHead className="w-16">类型</TableHead>
                                    <TableHead className="w-16">必填</TableHead>
                                    <TableHead>描述</TableHead>
                                  </TableRow>
                                </TableHeader>
                                <TableBody>
                                  {ep.parameters.map((param) => (
                                    <TableRow key={param.name}>
                                      <TableCell>
                                        <code className="font-mono text-xs">{param.name}</code>
                                      </TableCell>
                                      <TableCell>
                                        <Badge variant="outline" className="text-xs">
                                          {param.location}
                                        </Badge>
                                      </TableCell>
                                      <TableCell>
                                        <span className="font-mono text-xs text-muted-foreground">
                                          {param.type}
                                        </span>
                                      </TableCell>
                                      <TableCell>
                                        {param.required ? (
                                          <span className="text-xs text-destructive">是</span>
                                        ) : (
                                          <span className="text-xs text-muted-foreground">否</span>
                                        )}
                                      </TableCell>
                                      <TableCell className="text-sm text-muted-foreground">
                                        {param.description}
                                        {param.example && (
                                          <span className="ml-1 text-xs opacity-60">
                                            (示例：{param.example})
                                          </span>
                                        )}
                                      </TableCell>
                                    </TableRow>
                                  ))}
                                </TableBody>
                              </Table>
                            </div>
                          )}

                          {ep.responseExample && (
                            <div>
                              <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                                响应示例
                              </h4>
                              <pre className="overflow-x-auto rounded-md border bg-muted/50 p-3 font-mono text-xs leading-relaxed">
                                <code>{ep.responseExample}</code>
                              </pre>
                            </div>
                          )}
                        </CardContent>
                      )}
                    </Card>
                  ))}
                </div>
              </section>
            ))}

            {/* OpenAPI spec link */}
            <Card className="border-dashed">
              <CardContent className="pt-4">
                <p className="text-sm text-muted-foreground">
                  机器可读规范：{" "}
                  <a
                    href="/v1/openapi.json"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary underline underline-offset-4 hover:no-underline"
                  >
                    GET /v1/openapi.json
                  </a>
                  {" "}（OpenAPI 3.1 JSON，可导入 Postman / Insomnia）
                </p>
              </CardContent>
            </Card>
        </main>

        {/* ── Right TOC ── */}
        <aside className="hidden xl:block w-48 shrink-0 border-l">
          <div className="sticky top-16 overflow-y-auto py-8 pl-5 pr-4" style={{ maxHeight: "calc(100vh - 4rem)" }}>
            <p className="mb-3 text-[11px] font-semibold uppercase tracking-widest text-muted-foreground/50">
              本页目录
            </p>
            <nav>
              <ul className="space-y-2.5">
                {API_GROUPS.map((group) => (
                  <li key={group.id}>
                    <a
                      href={`#group-${group.id}`}
                      className="block text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
                    >
                      {group.label}
                    </a>
                    <ul className="mt-1 ml-2 space-y-0.5 border-l border-border pl-2.5">
                      {group.endpoints.map((ep) => (
                        <li key={ep.id}>
                          <a
                            href={`#ep-${ep.id}`}
                            className="block py-0.5 text-[11px] text-muted-foreground/60 hover:text-muted-foreground transition-colors leading-snug"
                          >
                            {ep.summary}
                          </a>
                        </li>
                      ))}
                    </ul>
                  </li>
                ))}
              </ul>
            </nav>
          </div>
        </aside>

      </div>
    </div>
  )
}
