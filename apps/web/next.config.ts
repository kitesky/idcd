import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  output: 'standalone',
  typedRoutes: true,
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
  }
}

export default nextConfig