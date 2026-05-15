import nextConfig from "eslint-config-next/core-web-vitals"

export default [
  ...nextConfig,
  {
    settings: {
      "import/resolver": {
        typescript: { project: "./tsconfig.json" }
      }
    },
    rules: {
      "import/no-unresolved": "error",
      "import/named": "error",
      "import/no-duplicates": "error",
      "import/no-cycle": ["warn", { maxDepth: 5 }]
    }
  }
]
