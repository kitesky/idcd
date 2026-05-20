import type { NextConfig } from 'next'
import createNextIntlPlugin from 'next-intl/plugin'
import createMDX from '@next/mdx'

const withNextIntl = createNextIntlPlugin('./i18n.ts')

// MDX 用作内容源（非 page 扩展）。文档动态从 src/content/docs/{slug}/{locale}.mdx
// import，所以这里不把 mdx 加入 pageExtensions，避免误把 content/* 当作页面路由。
const withMDX = createMDX({
  extension: /\.mdx?$/,
  options: {
    // 用空 array 显式声明，未来需要 remark/rehype 插件时在这里加。
    remarkPlugins: [],
    rehypePlugins: [],
  },
})

const nextConfig: NextConfig = {
  // output: 'standalone' — 改用 @opennextjs/cloudflare，不需要 standalone 模式
  // typedRoutes: true,  // 77 页面时内存开销过大，仅在 CI build 中开启
  // pageExtensions 不含 mdx：MDX 走动态 import，避免 content/* 被识别为路由
  pageExtensions: ['ts', 'tsx'],
  // Turbopack 默认支持 .mdx，dev 模式自动应用 createMDX 配置；
  // production build (webpack) 由 withMDX 包装注入 loader。
  // 显式把 Turbopack root 和 file tracing root 锁在本目录，
  // 禁止向 monorepo 上层扫描（Go 二进制 / worktree / docs 会撑爆 dev 内存导致 OOM）。
  turbopack: {
    root: __dirname,
  },
  outputFileTracingRoot: __dirname,
  images: {
    remotePatterns: [
      {
        protocol: 'https',
        hostname: 'idcd.com',
      },
      {
        protocol: 'https',
        hostname: '*.idcd.com',
      }
    ]
  },
  async headers() {
    return [
      {
        source: '/(.*)',
        headers: [
          {
            key: 'X-DNS-Prefetch-Control',
            value: 'on'
          },
          {
            key: 'X-Frame-Options',
            value: 'SAMEORIGIN'
          },
          {
            key: 'X-Content-Type-Options',
            value: 'nosniff'
          },
          {
            key: 'Referrer-Policy',
            value: 'strict-origin-when-cross-origin'
          },
          {
            key: 'Permissions-Policy',
            value: 'camera=(), microphone=(), geolocation=(), payment=()'
          }
        ]
      }
    ]
  }
}

export default withNextIntl(withMDX(nextConfig))