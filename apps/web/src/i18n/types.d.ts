// next-intl global IntlMessages type augmentation.
//
// Phase 5.2 — drive compile-time validation of `t('namespace.key.path')` calls
// by giving next-intl a typed view of every namespace message tree.
//
// Convention: `cn` is the authoritative schema (Phase 5.1 lint-i18n.ts already
// enforces cn↔other-locale key parity), so we infer the shape from cn JSON.
// Adding a new namespace = drop the JSON in messages/cn/<ns>.json + append a
// new field below. Namespaces declared in request.ts but lacking a cn JSON
// file (e.g. footer, statusPages, enums, validation, transparency, legal) are
// intentionally omitted here — they have no authoritative schema yet.
//
// See https://next-intl.dev/docs/workflows/typescript

import type aboutMessages from './messages/cn/about.json'
import type adminMessages from './messages/cn/admin.json'
import type alertsMessages from './messages/cn/alerts.json'
import type authMessages from './messages/cn/auth.json'
import type billingMessages from './messages/cn/billing.json'
import type commonMessages from './messages/cn/common.json'
import type dashboardMessages from './messages/cn/dashboard.json'
import type docsMessages from './messages/cn/docs.json'
import type errorsMessages from './messages/cn/errors.json'
import type homeMessages from './messages/cn/home.json'
import type incidentsMessages from './messages/cn/incidents.json'
import type leaderboardMessages from './messages/cn/leaderboard.json'
import type monitorsMessages from './messages/cn/monitors.json'
import type navMessages from './messages/cn/nav.json'
import type nodesMessages from './messages/cn/nodes.json'
import type pricingMessages from './messages/cn/pricing.json'
import type settingsMessages from './messages/cn/settings.json'
import type statusMessages from './messages/cn/status.json'
import type toolsMessages from './messages/cn/tools.json'
import type userMenuMessages from './messages/cn/userMenu.json'

declare global {
  // next-intl looks up this ambient interface to type `useTranslations`,
  // `getTranslations`, and `t(...)` keys.
  interface IntlMessages {
    about: typeof aboutMessages
    admin: typeof adminMessages
    alerts: typeof alertsMessages
    auth: typeof authMessages
    billing: typeof billingMessages
    common: typeof commonMessages
    dashboard: typeof dashboardMessages
    docs: typeof docsMessages
    errors: typeof errorsMessages
    home: typeof homeMessages
    incidents: typeof incidentsMessages
    leaderboard: typeof leaderboardMessages
    monitors: typeof monitorsMessages
    nav: typeof navMessages
    nodes: typeof nodesMessages
    pricing: typeof pricingMessages
    settings: typeof settingsMessages
    status: typeof statusMessages
    tools: typeof toolsMessages
    userMenu: typeof userMenuMessages
  }
}

export {}
