/**
 * Sentry 前端初始化（S1 占位）
 *
 * S1 仅为占位代码，不引入真实 @sentry/nextjs SDK 依赖。
 * S2 需要在 Sentry 控制台创建项目获取 DSN 后，启用真实集成。
 */

/**
 * 初始化 Sentry 前端监控
 *
 * S1: 仅输出日志占位，不加载 SDK
 * S2: 启用真实 Sentry.init() 调用
 *
 * 使用方式：
 * 在 apps/web/src/app/layout.tsx 或 _app.tsx 的 useEffect 中调用
 */
export function initSentry() {
  // 仅在浏览器环境执行
  if (typeof window === "undefined") return

  // 检查环境变量（S2 需配置）
  const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN
  if (!dsn) {
    console.log("[Sentry] S1 placeholder - DSN not configured")
    return
  }

  // TODO: S2 启用真实 Sentry SDK
  // import * as Sentry from "@sentry/nextjs"
  // Sentry.init({
  //   dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
  //   tracesSampleRate: 0.1,
  //   environment: process.env.NODE_ENV,
  // })

  console.log("[Sentry] S1 placeholder - SDK not loaded")
}
