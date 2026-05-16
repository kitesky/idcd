import type { Metadata } from "next"

export const metadata: Metadata = { title: "DNS 解析 | idcd 工具文档" }

export default function DnsToolPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>DNS 解析</h1>
      <p>从多个地区查询 DNS 解析结果，检测 DNS 污染、劫持或解析不一致问题。</p>

      <h2>功能说明</h2>
      <ul>
        <li>支持 A、AAAA、MX、TXT、CNAME、NS、SOA 等记录类型</li>
        <li>从多个地区的 DNS 服务器同时查询</li>
        <li>对比不同地区的解析结果，定位污染或劫持</li>
        <li>显示 TTL 值</li>
      </ul>

      <h2>API 参数</h2>
      <table>
        <thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>domain</code></td><td>string</td><td>是</td><td>查询的域名</td></tr>
          <tr><td><code>type</code></td><td>string</td><td>否</td><td>记录类型，默认 A</td></tr>
          <tr><td><code>nodes</code></td><td>string[]</td><td>否</td><td>指定查询节点</td></tr>
        </tbody>
      </table>

      <h2>示例</h2>
      <h3>查询 A 记录</h3>
      <pre><code>{`curl -X POST https://api.idcd.com/v1/tools/dns \\
  -H "Authorization: Bearer sk_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{"domain": "example.com", "type": "A"}'`}</code></pre>

      <h3>查询 MX 记录</h3>
      <pre><code>{`{
  "domain": "gmail.com",
  "type": "MX",
  "nodes": ["beijing", "tokyo", "us-east"]
}`}</code></pre>

      <h2>典型使用场景</h2>
      <ul>
        <li>验证 DNS 记录是否在全球生效（新域名解析传播）</li>
        <li>检测国内外解析结果是否一致（CDN 分流）</li>
        <li>排查邮件发送问题（MX/SPF/DKIM 记录）</li>
      </ul>

      <h2>在线工具</h2>
      <p>无需 API Key，直接在 <a href="/tools/dns">在线工具</a> 中试用。</p>
    </article>
  )
}
