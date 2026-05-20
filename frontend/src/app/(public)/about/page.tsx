import type { Metadata } from "next"
import Link from "next/link"
import { Button, Card, CardContent, CardHeader, CardTitle } from "@/components/ui"
import { ArrowLeft, Globe, Zap, Shield, GitBranch, Mail } from "lucide-react"
import { getT } from "@/i18n/getT"
import { getLocale } from "@/i18n/locale"
import { generateAlternates } from "@/lib/seo"

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getLocale()
  const t = await getT('about', locale)
  return {
    title: t('meta.title'),
    description: t('meta.description'),
    alternates: generateAlternates('/about', locale),
  }
}

export default async function AboutPage() {
  const locale = await getLocale()
  const t = await getT('about', locale)
  const tCommon = await getT('common', locale)

  return (
    <main className="flex-1">
      <div className="mx-auto max-w-4xl px-4 py-12 sm:px-6 lg:px-8">
        {/* 返回按钮 */}
        <div className="mb-8">
          <Link href="/">
            <Button variant="ghost" size="sm" className="gap-2">
              <ArrowLeft className="h-4 w-4" />
              {tCommon('back')}
            </Button>
          </Link>
        </div>

        {/* 标题 */}
        <div className="mb-12 text-center">
          <h1 className="text-4xl font-bold tracking-tight mb-4">{t('hero.title')}</h1>
          <p className="text-xl text-muted-foreground max-w-2xl mx-auto">
            {t('hero.subtitle')}
          </p>
        </div>

        {/* 核心特性 */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-12">
          <Card>
            <CardHeader>
              <Globe className="h-8 w-8 text-primary mb-2" />
              <CardTitle className="text-lg">{t('features.global.title')}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                {t('features.global.description')}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <Zap className="h-8 w-8 text-primary mb-2" />
              <CardTitle className="text-lg">{t('features.diagnostics.title')}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                {t('features.diagnostics.description')}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <Shield className="h-8 w-8 text-primary mb-2" />
              <CardTitle className="text-lg">{t('features.evidence.title')}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                {t('features.evidence.description')}
              </p>
            </CardContent>
          </Card>
        </div>

        {/* 产品简介 */}
        <Card className="mb-8">
          <CardHeader>
            <CardTitle>{t('intro.title')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-muted-foreground">
            <p>{t('intro.p1')}</p>
            <p>{t('intro.p2')}</p>
            <ul className="list-disc list-inside space-y-2 ml-4">
              <li>
                <strong>{t('intro.featureList.probe.label')}</strong>
                {t('intro.featureList.probe.desc')}
              </li>
              <li>
                <strong>{t('intro.featureList.tools.label')}</strong>
                {t('intro.featureList.tools.desc')}
              </li>
              <li>
                <strong>{t('intro.featureList.monitor.label')}</strong>
                {t('intro.featureList.monitor.desc')}
              </li>
              <li>
                <strong>{t('intro.featureList.diagnose.label')}</strong>
                {t('intro.featureList.diagnose.desc')}
              </li>
              <li>
                <strong>{t('intro.featureList.evidence.label')}</strong>
                {t('intro.featureList.evidence.desc')}
              </li>
              <li>
                <strong>{t('intro.featureList.utils.label')}</strong>
                {t('intro.featureList.utils.desc')}
              </li>
            </ul>
            <p>{t('intro.p3')}</p>
          </CardContent>
        </Card>

        {/* 技术说明 */}
        <Card className="mb-8">
          <CardHeader>
            <CardTitle>{t('tech.title')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-muted-foreground">
            <p>{t('tech.p1')}</p>
            <ul className="list-disc list-inside space-y-2 ml-4">
              <li><strong>{t('tech.stack.backend.label')}</strong>{t('tech.stack.backend.desc')}</li>
              <li><strong>{t('tech.stack.frontend.label')}</strong>{t('tech.stack.frontend.desc')}</li>
              <li><strong>{t('tech.stack.db.label')}</strong>{t('tech.stack.db.desc')}</li>
              <li><strong>{t('tech.stack.cache.label')}</strong>{t('tech.stack.cache.desc')}</li>
              <li><strong>{t('tech.stack.security.label')}</strong>{t('tech.stack.security.desc')}</li>
              <li><strong>{t('tech.stack.nodes.label')}</strong>{t('tech.stack.nodes.desc')}</li>
            </ul>
            <p className="mt-4">{t('tech.p2')}</p>
          </CardContent>
        </Card>

        {/* 联系方式 */}
        <Card>
          <CardHeader>
            <CardTitle>{t('contact.title')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center gap-3 text-muted-foreground">
                <Mail className="h-5 w-5 text-primary" />
                <div>
                  <p className="text-sm font-medium text-foreground">{t('contact.email')}</p>
                  <a
                    href="mailto:kite365@gmail.com"
                    className="text-sm text-primary hover:underline"
                  >
                    kite365@gmail.com
                  </a>
                </div>
              </div>

              <div className="flex items-center gap-3 text-muted-foreground">
                <Globe className="h-5 w-5 text-primary" />
                <div>
                  <p className="text-sm font-medium text-foreground">{t('contact.website')}</p>
                  <a
                    href="https://idcd.com"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-primary hover:underline"
                  >
                    https://idcd.com
                  </a>
                </div>
              </div>

              <div className="flex items-center gap-3 text-muted-foreground">
                <GitBranch className="h-5 w-5 text-primary" />
                <div>
                  <p className="text-sm font-medium text-foreground">{t('contact.github')}</p>
                  <a
                    href="https://github.com/kite365/idcd"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-primary hover:underline"
                  >
                    https://github.com/kite365/idcd
                  </a>
                </div>
              </div>
            </div>

            <div className="mt-6 pt-6 border-t">
              <p className="text-sm text-muted-foreground">
                {t('contact.footer')}
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </main>
  )
}
