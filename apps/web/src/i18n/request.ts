import { getRequestConfig } from 'next-intl/server'
import { headers } from 'next/headers'
import { defaultLocale, isValidLocale, type Locale } from './routing'

async function loadMessages(locale: Locale) {
  if (locale === 'en') {
    const [nav, tools, auth, home, leaderboard, nodes, pricing, errors, common,
      monitors, alerts, settings, billing, dashboard, status, admin] = await Promise.all([
      import('./messages/en/nav.json'),
      import('./messages/en/tools.json'),
      import('./messages/en/auth.json'),
      import('./messages/en/home.json'),
      import('./messages/en/leaderboard.json'),
      import('./messages/en/nodes.json'),
      import('./messages/en/pricing.json'),
      import('./messages/en/errors.json'),
      import('./messages/en/common.json'),
      import('./messages/en/monitors.json'),
      import('./messages/en/alerts.json'),
      import('./messages/en/settings.json'),
      import('./messages/en/billing.json'),
      import('./messages/en/dashboard.json'),
      import('./messages/en/status.json'),
      import('./messages/en/admin.json'),
    ])
    return {
      nav: nav.default, tools: tools.default, auth: auth.default,
      home: home.default, leaderboard: leaderboard.default, nodes: nodes.default,
      pricing: pricing.default, errors: errors.default, common: common.default,
      monitors: monitors.default, alerts: alerts.default, settings: settings.default,
      billing: billing.default, dashboard: dashboard.default, status: status.default,
      admin: admin.default,
    }
  }

  const [nav, tools, auth, home, leaderboard, nodes, pricing, errors, common,
    monitors, alerts, settings, billing, dashboard, status, admin] = await Promise.all([
    import('./messages/zh/nav.json'),
    import('./messages/zh/tools.json'),
    import('./messages/zh/auth.json'),
    import('./messages/zh/home.json'),
    import('./messages/zh/leaderboard.json'),
    import('./messages/zh/nodes.json'),
    import('./messages/zh/pricing.json'),
    import('./messages/zh/errors.json'),
    import('./messages/zh/common.json'),
    import('./messages/zh/monitors.json'),
    import('./messages/zh/alerts.json'),
    import('./messages/zh/settings.json'),
    import('./messages/zh/billing.json'),
    import('./messages/zh/dashboard.json'),
    import('./messages/zh/status.json'),
    import('./messages/zh/admin.json'),
  ])
  return {
    nav: nav.default, tools: tools.default, auth: auth.default,
    home: home.default, leaderboard: leaderboard.default, nodes: nodes.default,
    pricing: pricing.default, errors: errors.default, common: common.default,
    monitors: monitors.default, alerts: alerts.default, settings: settings.default,
    billing: billing.default, dashboard: dashboard.default, status: status.default,
    admin: admin.default,
  }
}

export default getRequestConfig(async ({ requestLocale }) => {
  const headersList = await headers()
  const headerLocale = headersList.get('x-locale') ?? ''
  const rawLocale = (await requestLocale) ?? headerLocale
  const locale: Locale = isValidLocale(rawLocale) ? rawLocale : defaultLocale

  return {
    locale,
    messages: await loadMessages(locale),
  }
})
