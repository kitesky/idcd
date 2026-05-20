import {
  Activity,
  Globe,
  Code2,
  Monitor,
  BellRing,
  BarChart2,
  Zap,
  Webhook,
} from "lucide-react"
import Link from "next/link"
import { useTranslations } from "next-intl"
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Badge } from "@/components/ui"
import { HeroSearch } from "@/components/hero-search"

const FEATURE_KEYS = [
  { icon: Activity, key: "networkMonitor" },
  { icon: Globe, key: "dnsMonitor" },
  { icon: Code2, key: "apiMonitor" },
  { icon: Monitor, key: "webMonitor" },
  { icon: BellRing, key: "alerts" },
  { icon: BarChart2, key: "analysis" },
  { icon: Zap, key: "instant" },
  { icon: Webhook, key: "openApi" },
] as const

const TOOL_KEYS = [
  { key: "http", href: "/tools/http" },
  { key: "ping", href: "/tools/ping" },
  { key: "dns", href: "/tools/dns" },
  { key: "ssl", href: "/tools/ssl" },
  { key: "traceroute", href: "/tools/traceroute" },
  { key: "diagnose", href: "/tools/diagnose" },
] as const

export default function HomePage() {
  const t = useTranslations("home")

  return (
    <main className="flex-1">
      {/* Hero Section — HeroSearch 独立组件 */}
      <HeroSearch />

      {/* Features Section */}
      <section className="py-12 md:py-16 border-b">
        <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
          <h2 className="text-center text-2xl font-bold text-foreground mb-8">
            {t("features.title")}
          </h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {FEATURE_KEYS.map(({ icon: Icon, key }) => (
              <Card key={key} className="p-6">
                <div className="flex flex-col items-center text-center">
                  <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
                    <Icon className="h-6 w-6 text-primary" />
                  </div>
                  <h3 className="text-sm font-semibold text-foreground">
                    {t(`features.${key}.title`)}
                  </h3>
                  <p className="text-xs text-muted-foreground mt-2">
                    {t(`features.${key}.desc`)}
                  </p>
                </div>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* Tools Section */}
      <section className="py-12 md:py-16 bg-muted/30">
        <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
          <div className="text-center mb-10">
            <h2 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              {t("tools.title")}
            </h2>
          </div>

          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {TOOL_KEYS.map(({ key, href }) => (
              <Card key={key} className="group hover:shadow-lg transition-shadow">
                <CardHeader>
                  <div className="flex items-center gap-3">
                    <Badge variant="outline">{t(`tools.${key}.name`)}</Badge>
                  </div>
                  <CardTitle className="text-lg">{t(`tools.${key}.name`)}</CardTitle>
                </CardHeader>
                <CardContent>
                  <CardDescription className="mb-4">
                    {t(`tools.${key}.desc`)}
                  </CardDescription>
                  <Button asChild variant="outline" className="w-full">
                    <Link href={href}>{t("tools.useTool")}</Link>
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>
    </main>
  )
}
