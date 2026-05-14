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
    <div className="flex min-h-screen flex-col">
      {/* Page header */}
      <div className="border-b bg-background/95 py-10">
        <div className="container max-w-6xl">
          <h1 className="text-3xl font-bold tracking-tight">API 参考</h1>
          <p className="mt-2 text-muted-foreground">
            idcd 全球网络诊断 API 完整参考文档。Base URL：{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-sm">
              https://api.idcd.com/v1
            </code>
          </p>
          <div className="mt-4 flex flex-wrap gap-2 text-sm text-muted-foreground">
            <span className="flex items-center gap-1">
              <span className="inline-block h-2 w-2 rounded-full bg-blue-400" /> GET
            </span>
            <span className="flex items-center gap-1">
              <span className="inline-block h-2 w-2 rounded-full bg-green-400" /> POST
            </span>
            <span className="flex items-center gap-1">
              <span className="inline-block h-2 w-2 rounded-full bg-yellow-400" /> PATCH
            </span>
            <span className="flex items-center gap-1">
              <span className="inline-block h-2 w-2 rounded-full bg-red-400" /> DELETE
            </span>
          </div>
        </div>
      </div>

      <div className="container max-w-6xl flex-1 py-8">
        <div className="flex gap-8">
          {/* Left sidebar navigation */}
          <aside className="hidden w-52 shrink-0 lg:block">
            <div className="sticky top-8 overflow-y-auto" style={{ maxHeight: "calc(100vh - 8rem)" }}>
              <nav aria-label="API 端点导航">
                <ul className="space-y-1">
                  {API_GROUPS.map((group) => (
                    <li key={group.id}>
                      <a
                        href={`#group-${group.id}`}
                        className="block rounded-md px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                      >
                        {group.label}
                      </a>
                      <ul className="ml-3 mt-0.5 space-y-0.5">
                        {group.endpoints.map((ep) => (
                          <li key={ep.id}>
                            <a
                              href={`#ep-${ep.id}`}
                              className="block rounded px-2 py-1 text-xs text-muted-foreground/70 transition-colors hover:text-muted-foreground"
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

          {/* Main content */}
          <main className="min-w-0 flex-1 space-y-12">
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
        </div>
      </div>
    </div>
  )
}
