import type { Metadata } from 'next'
import { getTranslations } from 'next-intl/server'
import { Button } from '@/components/ui'

export const metadata: Metadata = {
  title: 'idcd — Global Network Diagnostics Tools',
  description: 'Professional network diagnostics and monitoring platform. Multi-region ping, HTTP checks, DNS lookup, traceroute, SSL checks and more — free and instant.',
  keywords: ['network diagnostics', 'ping test', 'http check', 'dns lookup', 'traceroute', 'ssl check', 'global probe'],
  alternates: {
    canonical: 'https://idcd.com/en',
    languages: {
      zh: 'https://idcd.com/',
      en: 'https://idcd.com/en',
    },
  },
  openGraph: {
    title: 'idcd — Global Network Diagnostics',
    description: 'Multi-node probing, one-click diagnostics',
    url: 'https://idcd.com/en',
    siteName: 'idcd',
    type: 'website',
  },
}

const FEATURED_TOOL_SLUGS = [
  'ping',
  'http',
  'dns',
  'traceroute',
  'ssl',
  'ip',
  'whois',
  'icp',
  'diagnose',
  'ipv6-check',
] as const

export default async function EnglishHomePage() {
  const t = await getTranslations({ locale: 'en', namespace: 'tools' })

  return (
    <main className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-16 max-w-5xl">
        <div className="text-center mb-12">
          <h1 className="text-4xl font-bold tracking-tight mb-4">
            Global Network Diagnostics
          </h1>
          <p className="text-lg text-muted-foreground max-w-2xl mx-auto">
            Free, instant network tools powered by 100+ real probe nodes worldwide.
            Diagnose latency, DNS, SSL, and connectivity issues in seconds.
          </p>
          <div className="mt-6 flex gap-3 justify-center">
            <Button asChild>
              <a href="/en/tools/ping">Try Ping Check</a>
            </Button>
            <Button variant="outline" asChild>
              <a href="/leaderboard">CDN Leaderboard</a>
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
          {FEATURED_TOOL_SLUGS.map((slug) => (
            <a
              key={slug}
              href={`/en/tools/${slug}`}
              className="group rounded-lg border bg-card p-5 hover:border-primary transition-colors"
            >
              <h2 className="font-semibold text-base mb-1 group-hover:text-primary transition-colors">
                {t(`${slug}.title`)}
              </h2>
              <p className="text-sm text-muted-foreground">
                {t(`${slug}.description`)}
              </p>
            </a>
          ))}
        </div>
      </div>
    </main>
  )
}
