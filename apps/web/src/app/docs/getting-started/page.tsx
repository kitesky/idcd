import type { Metadata } from "next"
import Link from "next/link"

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
        <li>访问 <Link href="/tools">在线工具</Link> 页面</li>
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

      <hr />

      <h2>步骤 1：创建你的第一个监控</h2>
      <ol>
        <li>
          登录后进入 <Link href="/app/monitors">监控列表</Link>，点击右上角&quot;创建监控&quot;按钮
        </li>
        <li>
          选择监控类型：
          <ul>
            <li><strong>HTTP/HTTPS</strong>：检测 API 或网站可用性（最常用）</li>
            <li><strong>Ping</strong>：检测服务器网络可达性</li>
            <li><strong>TCP</strong>：检测指定端口连通性</li>
            <li><strong>SSL 到期</strong>：监控 HTTPS 证书即将到期告警</li>
          </ul>
        </li>
        <li>
          填写目标地址，例如 <code>https://api.example.com/health</code>
        </li>
        <li>
          设置检测频率（推荐 <strong>5 分钟</strong>，高可用业务可选 1 分钟）
        </li>
        <li>
          高级配置（可选）：
          <ul>
            <li>断言状态码：期望返回 <code>200</code></li>
            <li>关键字匹配：响应体必须包含指定字符串</li>
            <li>超时时间：单次拨测最长等待时间（默认 10s）</li>
          </ul>
        </li>
      </ol>

      <h2>步骤 2：配置告警通道</h2>
      <ol>
        <li>
          进入 <a href="/app/alerts">告警中心</a> → 告警通道标签页
        </li>
        <li>
          点击&quot;添加通道&quot;，选择通知方式：
          <ul>
            <li><strong>Email</strong>：直接发送到你的邮箱</li>
            <li><strong>企业微信</strong>：通过 Webhook 机器人推送</li>
            <li><strong>飞书</strong>：通过飞书自定义机器人推送</li>
            <li><strong>Webhook</strong>：自定义 HTTP 回调，接入任意系统</li>
          </ul>
        </li>
        <li>
          点击通道右侧的&quot;测试&quot;按钮，确认通道可正常接收消息
        </li>
        <li>
          创建告警策略：绑定监控项 + 通知通道，并设置告警延迟（0 = 立即告警，建议设 2-3 分钟避免误报）
        </li>
      </ol>

      <h2>步骤 3：查看监控数据</h2>
      <ul>
        <li>
          <strong>仪表盘</strong>：总体健康状态概览，置顶关键监控项
        </li>
        <li>
          <strong>监控详情</strong>：24 小时延迟趋势图、平均响应时间、在线率统计
        </li>
        <li>
          <strong>故障记录</strong>：查看历史故障时间线，生成复盘报告
        </li>
        <li>
          <strong>SLA 月报</strong>：在 <a href="/app/reports">报告中心</a> 下载 PDF 或 CSV 格式的月度可用性报告
        </li>
      </ul>

      <h2>API 接入（开发者）</h2>
      <ol>
        <li>
          在 <a href="/app/settings/api-keys">设置 → API Keys</a> 页面创建你的 API Key
        </li>
        <li>
          在请求头中携带 Key：
          <br />
          <code>Authorization: Bearer &lt;your-key&gt;</code>
        </li>
        <li>
          查阅完整接口文档：<a href="/docs/api">API 参考</a>
        </li>
      </ol>

      <h2>常见问题</h2>
      <dl>
        <dt><strong>Q：监控显示 DOWN 但实际服务正常？</strong></dt>
        <dd>
          检查节点选择范围，可能是特定地区网络波动导致。可尝试增大超时时间，或切换到距离目标服务更近的节点。
        </dd>

        <dt><strong>Q：收到太多告警误报怎么办？</strong></dt>
        <dd>
          在告警策略中调整&quot;延迟分钟数&quot;，例如设为 3 分钟，即连续失败超过 3 分钟才触发通知，可有效过滤短暂抖动。
        </dd>

        <dt><strong>Q：如何对外共享服务状态？</strong></dt>
        <dd>
          在 <Link href="/app/status-pages">状态页</Link> 中创建公开状态页，可自定义域名，适合对用户或客户公开服务健康状况。
        </dd>

        <dt><strong>Q：多人团队如何协作？</strong></dt>
        <dd>
          在 <a href="/app/settings">设置 → 团队成员</a> 中邀请成员，不同角色拥有不同的查看与操作权限。
        </dd>
      </dl>
    </article>
  )
}
