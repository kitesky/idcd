# admin namespace EN translations

Admin backend is VPN-internal. Per I18N-PLAN.md §3 Phase 4b decision D2:

- **Architecture i18n key is mandatory** — every UI string in `app/admin/*`
  passes through `t(...)` so the system supports EN structurally.
- **Partial EN translation only** — high-frequency nav and buttons are
  translated here. All other keys are intentionally absent. The fallback chain
  in `apps/web/src/i18n/request.ts` (`loadNamespace` deep-merge) auto-fills
  every missing key from `cn/admin.json`, so the admin UI renders without
  broken `t(...)` placeholders when an internal operator switches to EN.

To complete EN translation later, just add the missing keys to
`messages/en/admin.json`. No code changes needed — the deep-merge picks them
up automatically.

To audit current coverage, diff the key sets:

```bash
jq -r 'paths(scalars) | join(".")' apps/web/src/i18n/messages/cn/admin.json | sort > /tmp/cn.keys
jq -r 'paths(scalars) | join(".")' apps/web/src/i18n/messages/en/admin.json | sort > /tmp/en.keys
diff /tmp/cn.keys /tmp/en.keys
```
