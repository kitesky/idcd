import type { Metadata } from "next"

export const metadata: Metadata = { title: "快速开始 | idcd 文档" }

export default function GettingStartedPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>快速开始</h1>

      <h2>什么是 idcd？</h2>
      <p>idcd 是一个多节点网络诊断工具平台，提供全球分布式拨测服务。通过 idcd，你可以：</p>
      <ul>
        <li>从全球多个节点测试你的服务可达性</li>
        <li>获取真实的网络延迟和性能数据</li>
        <li>快速定位网络问题的地理位置</li>
        <li>生成可信的网络诊断证据</li>
      </ul>

      <h2>节点分布</h2>
      <p>idcd 在全球多个地区部署了测试节点：</p>
      <ul>
        <li><strong>亚洲</strong>：中国大陆（北京、上海、广州）、香港、日本、新加坡</li>
        <li><strong>北美</strong>：美国东西海岸</li>
        <li><strong>欧洲</strong>：英国、德国、法国</li>
      </ul>

      <h2>快速使用</h2>
      <ol>
        <li>访问 <a href="/tools">在线工具</a> 页面</li>
        <li>选择需要的工具（如 HTTP 拨测、Ping 测试）</li>
        <li>输入目标域名或 IP</li>
        <li>点击开始，等待全球节点返回结果</li>
      </ol>

      <h2>API 接入</h2>
      <p>idcd 提供 REST API 和 MCP Server，方便程序化接入：</p>
      <ul>
        <li><a href="/docs/mcp">MCP Server</a> — 让 Claude、Cursor 等 AI 直接调用拨测能力</li>
        <li>REST API — 参考 API 文档（即将发布）</li>
      </ul>
    </article>
  )
}
