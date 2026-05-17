import nextConfig from "eslint-config-next/core-web-vitals"

// 设计原则
// ─────────
// 严格 = 任何能在 dev 启动期把内存打爆、或在生产期默默吞错的模式
//        必须停在编译期。不做风格洁癖。
//
// 改动来历:
//   - 2026-05-16 OOM 复盘：admin/{beta-invitations,upgrades} 的 *-client.tsx
//     反向 import "./page" 形成跨 RSC/client 边界循环，Turbopack 启动几秒内
//     RSS 狂飙被 kill。原 import/no-cycle 是 warn，dev 启动也不跑 lint —— 全程
//     无拦截。本配置把这一类全部升 error，并直接禁 "./page" / "../page" 反向
//     import 这条具体模式。
const config = [
  ...nextConfig,
  {
    settings: {
      "import/resolver": {
        typescript: { project: "./tsconfig.json" }
      }
    },
    rules: {
      // ── 启动 / 构建期 OOM 防线 (绝不降级为 warn) ─────────────────────
      "import/no-cycle": ["error", { maxDepth: 5, ignoreExternal: true }],
      "import/no-self-import": "error",
      "import/no-unresolved": "error",
      "import/named": "error",
      "import/no-duplicates": "error",
      "import/no-relative-packages": "error",
      "import/no-useless-path-segments": ["error", { noUselessIndex: true }],

      // ── 禁 server↔client 反向 import (本次 OOM 直接根因) ────────────
      //   *-client.tsx 用 type 从 page.tsx 取定义 → page 被同时拖进
      //   Server 编译图和 client 编译图，Turbopack 来回 ping-pong 编译。
      //   共享类型应放到 ./types.ts，page.tsx 和 *-client.tsx 都从那里 import.
      "no-restricted-imports": ["error", {
        patterns: [
          {
            group: ["./page", "../page", "./page.tsx", "../page.tsx"],
            message: "禁止反向 import page.tsx (会与 page → *-client 形成 RSC/client 边界循环, 触发 Turbopack OOM)。把共享类型放到同目录 types.ts."
          },
          {
            group: ["./layout", "../layout", "./layout.tsx", "../layout.tsx"],
            message: "禁止反向 import layout.tsx，同样的 RSC/client 边界风险。共享类型放 types.ts."
          }
        ]
      }],

      // ── React Hooks: 错依赖 → 无限重渲染 → 实质 OOM ─────────────────
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "error",

      // ── TS 基础卫生 ──────────────────────────────────────────────
      "@typescript-eslint/no-unused-vars": ["error", {
        argsIgnorePattern: "^_",
        varsIgnorePattern: "^_",
        caughtErrorsIgnorePattern: "^_"
      }]
    }
  },

  // 测试 / 脚本 / 配置文件放宽 (大量 mock 和 any，强约束反而干扰)
  //
  // 关键豁免: no-restricted-imports 不适用于测试文件
  //   测试文件 import "../page" 是合法的 (渲染并断言)，不会形成 RSC/client
  //   运行时循环 (vitest 跑的是 Node 环境，没有 Turbopack 编译图)。
  {
    files: [
      "**/*.test.ts",
      "**/*.test.tsx",
      "**/__tests__/**",
      "vitest.config.ts",
      "next.config.ts",
      "postcss.config.mjs",
      "eslint.config.mjs",
      "scripts/**"
    ],
    rules: {
      "@typescript-eslint/no-unused-vars": "off",
      "import/no-unresolved": "off",
      "no-restricted-imports": "off"
    }
  }
]

export default config
