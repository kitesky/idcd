import { defineCloudflareConfig } from "@opennextjs/cloudflare"

const config = defineCloudflareConfig()
// Skip the heavy prebuild (lint/type-check/i18n) for local wrangler-dev runs;
// production deploys still go through `pnpm run build` via CI.
config.buildCommand = "pnpm exec next build"

export default config
