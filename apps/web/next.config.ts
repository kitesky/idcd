import type { NextConfig } from 'next'
import createNextIntlPlugin from 'next-intl/plugin'

const withNextIntl = createNextIntlPlugin('./i18n.ts')

const nextConfig: NextConfig = {
  // output: 'standalone' — 改用 @opennextjs/cloudflare，不需要 standalone 模式
  // typedRoutes: true,  // 77 页面时内存开销过大，仅在 CI build 中开启
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

export default withNextIntl(nextConfig)