import type { Metadata } from "next"

export const metadata: Metadata = { title: "Python SDK 集成 | idcd MCP 文档" }

export default function PythonExamplePage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>Python SDK 集成</h1>
      <p>通过 Python 直接调用 idcd REST API 或 MCP Server，适合脚本自动化和数据分析场景。</p>

      <h2>方式一：直接调用 REST API（推荐）</h2>
      <p>无需安装额外依赖，使用标准 <code>requests</code> 库即可：</p>
      <pre><code>pip install requests</code></pre>

      <pre><code>{`import requests

API_KEY = "sk_live_your_key_here"
BASE_URL = "https://api.idcd.com/v1"

headers = {"Authorization": f"Bearer {API_KEY}"}

# Ping 检测
resp = requests.post(f"{BASE_URL}/tools/ping", headers=headers, json={
    "target": "google.com",
    "nodes": ["tokyo", "singapore", "us-east"],
    "count": 5,
})
result = resp.json()
print(result)

# HTTP 检测
resp = requests.post(f"{BASE_URL}/tools/http", headers=headers, json={
    "url": "https://example.com",
    "nodes": ["beijing", "shanghai"],
})
print(resp.json())

# 一键诊断
resp = requests.post(f"{BASE_URL}/tools/diagnose", headers=headers, json={
    "target": "example.com",
})
print(resp.json())`}</code></pre>

      <h2>方式二：通过 MCP Python Client</h2>
      <p>适合在 Claude API 应用中集成拨测能力：</p>
      <pre><code>pip install anthropic mcp</code></pre>

      <pre><code>{`import subprocess
import anthropic
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

async def run():
    server_params = StdioServerParameters(
        command="mcp",
        env={"IDCD_API_KEY": "sk_live_your_key_here"},
    )

    async with stdio_client(server_params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()

            # 列出可用工具
            tools = await session.list_tools()
            print([t.name for t in tools.tools])

            # 调用 ping 工具
            result = await session.call_tool("ping", {
                "target": "google.com",
                "nodes": ["tokyo", "singapore"],
            })
            print(result)

import asyncio
asyncio.run(run())`}</code></pre>

      <h2>API 认证</h2>
      <p>建议通过环境变量传递 API Key，不要硬编码在代码中：</p>
      <pre><code>{`import os
API_KEY = os.environ["IDCD_API_KEY"]`}</code></pre>

      <h2>更多示例</h2>
      <ul>
        <li>使用 <code>diagnose</code> 工具定期巡检域名健康状态</li>
        <li>将 <code>http</code> 拨测结果写入数据库，绘制可用性趋势</li>
        <li>结合 <code>ssl</code> 工具在证书到期前发送告警</li>
      </ul>
      <p>完整 API 参考见 <a href="/docs/mcp/tools">工具列表</a> 和 OpenAPI 文档。</p>
    </article>
  )
}
