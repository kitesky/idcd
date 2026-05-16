export interface ToolDef {
  slug: string
  /** @deprecated Use translations (tools namespace, `${slug}.title`) instead */
  name: string
  /** @deprecated Use translations (tools namespace, `${slug}.description`) instead */
  description: string
  category: 'probe' | 'utility'
}

export const ALL_TOOLS: ToolDef[] = [
  // ── 拨测工具（15 个）────────────────────────────────────────────────
  {
    slug: 'ssl',
    name: 'SSL 证书检查',
    description: '检查域名 SSL/TLS 证书有效期和配置',
    category: 'probe',
  },
  {
    slug: 'whois',
    name: 'WHOIS 查询',
    description: '查询域名注册信息和持有者',
    category: 'probe',
  },
  {
    slug: 'icp',
    name: 'ICP 备案查询',
    description: '查询域名 ICP 备案状态',
    category: 'probe',
  },
  {
    slug: 'ip',
    name: 'IP 地理位置',
    description: '查询 IP 地址的地理位置和 ASN 信息',
    category: 'probe',
  },
  {
    slug: 'tcp',
    name: 'TCP 端口测试',
    description: '测试 TCP 端口的连通性',
    category: 'probe',
  },
  {
    slug: 'mtr',
    name: 'MTR 路由测试',
    description: '结合 Ping 和 Traceroute 的网络诊断',
    category: 'probe',
  },
  {
    slug: 'smtp',
    name: 'SMTP 邮件测试',
    description: '测试 SMTP 邮件服务器连通性',
    category: 'probe',
  },
  {
    slug: 'rdns',
    name: '反向 DNS 查询',
    description: '查询 IP 的反向 DNS 记录（PTR）',
    category: 'probe',
  },
  {
    slug: 'asn',
    name: 'ASN 查询',
    description: '查询 AS 号对应的网络信息',
    category: 'probe',
  },
  {
    slug: 'mx',
    name: 'MX 记录查询',
    description: '查询域名邮件交换记录',
    category: 'probe',
  },
  {
    slug: 'spf',
    name: 'SPF 记录查询',
    description: '查询域名 SPF 邮件认证记录',
    category: 'probe',
  },
  {
    slug: 'dmarc',
    name: 'DMARC 记录查询',
    description: '查询域名 DMARC 邮件认证策略',
    category: 'probe',
  },
  {
    slug: 'ntp',
    name: 'NTP 服务测试',
    description: '测试 NTP 时间同步服务器',
    category: 'probe',
  },
  {
    slug: 'dkim',
    name: 'DKIM 密钥查询',
    description: '查询域名 DKIM 公钥记录',
    category: 'probe',
  },
  {
    slug: 'bgp',
    name: 'BGP 路由查询',
    description: '查询 IP 或前缀的 BGP 路由信息',
    category: 'probe',
  },
  {
    slug: 'speedtest',
    name: '网速测试',
    description: '测试网络下载和上传带宽速度',
    category: 'probe',
  },

  // ── 辅助工具（35 个）────────────────────────────────────────────────
  {
    slug: 'yaml-formatter',
    name: 'YAML 格式化',
    description: 'YAML 格式美化和验证',
    category: 'utility',
  },
  {
    slug: 'xml-formatter',
    name: 'XML 格式化',
    description: 'XML 格式美化和验证',
    category: 'utility',
  },
  {
    slug: 'url-encode',
    name: 'URL 编解码',
    description: 'URL 编码与解码',
    category: 'utility',
  },
  {
    slug: 'unicode',
    name: 'Unicode 转换',
    description: 'Unicode 字符查询与转换',
    category: 'utility',
  },
  {
    slug: 'jwt-decoder',
    name: 'JWT 解码',
    description: 'JWT Token 解析工具',
    category: 'utility',
  },
  {
    slug: 'regex-tester',
    name: '正则表达式',
    description: '正则表达式测试与调试',
    category: 'utility',
  },
  {
    slug: 'cron-parser',
    name: 'Cron 可视化',
    description: 'Cron 表达式解析与下次执行时间',
    category: 'utility',
  },
  {
    slug: 'cidr-calculator',
    name: 'CIDR 计算器',
    description: 'IP 子网和 CIDR 范围计算',
    category: 'utility',
  },
  {
    slug: 'ipv6-check',
    name: 'IPv6 检测',
    description: 'IPv6 地址检测与格式转换',
    category: 'utility',
  },
  {
    slug: 'color-picker',
    name: '颜色格式转换',
    description: '颜色格式互转（HEX/RGB/HSL）',
    category: 'utility',
  },
  {
    slug: 'markdown',
    name: 'Markdown 预览',
    description: 'Markdown 编辑与实时预览',
    category: 'utility',
  },
  {
    slug: 'diff',
    name: '文本对比',
    description: '文本内容逐行差异比较',
    category: 'utility',
  },
  {
    slug: 'word-counter',
    name: '字数统计',
    description: '文本字数、字符数、行数统计',
    category: 'utility',
  },
  {
    slug: 'password-gen',
    name: '密码生成器',
    description: '安全随机密码生成',
    category: 'utility',
  },
  {
    slug: 'uuid-gen',
    name: 'UUID 生成器',
    description: 'UUID v4 批量生成工具',
    category: 'utility',
  },
  {
    slug: 'number-convert',
    name: '进制转换',
    description: '数字进制（二/八/十/十六）互转',
    category: 'utility',
  },
  {
    slug: 'text-case',
    name: '大小写转换',
    description: '文本大小写批量转换',
    category: 'utility',
  },
  {
    slug: 'html-encode',
    name: 'HTML 实体编码',
    description: 'HTML 实体编码与解码',
    category: 'utility',
  },
  {
    slug: 'json-to-yaml',
    name: 'JSON ↔ YAML',
    description: 'JSON 与 YAML 格式互转',
    category: 'utility',
  },
  {
    slug: 'url-parser',
    name: 'URL 解析',
    description: 'URL 各部分解析展示',
    category: 'utility',
  },
  {
    slug: 'user-agent',
    name: 'UA 解析',
    description: 'User-Agent 字符串解析',
    category: 'utility',
  },
  {
    slug: 'http-status',
    name: 'HTTP 状态码',
    description: 'HTTP 响应状态码查询',
    category: 'utility',
  },
  {
    slug: 'mime-type',
    name: 'MIME 类型',
    description: '文件 MIME 类型查询',
    category: 'utility',
  },
  {
    slug: 'timezone',
    name: '时区转换',
    description: '全球时区时间转换',
    category: 'utility',
  },
  {
    slug: 'date-calc',
    name: '日期计算',
    description: '日期差值和加减计算',
    category: 'utility',
  },
  {
    slug: 'image-base64',
    name: '图片 Base64',
    description: '图片转 Base64 编码',
    category: 'utility',
  },
  {
    slug: 'csv-formatter',
    name: 'CSV 格式化',
    description: 'CSV 数据解析和表格展示',
    category: 'utility',
  },
  {
    slug: 'number-format',
    name: '数字格式化',
    description: '数字千分位和货币格式化',
    category: 'utility',
  },
  {
    slug: 'lorem',
    name: 'Lorem Ipsum',
    description: 'Lorem Ipsum 占位文本生成',
    category: 'utility',
  },
  {
    slug: 'line-sort',
    name: '行排序',
    description: '文本行排序（正序/倒序）',
    category: 'utility',
  },
  {
    slug: 'duplicate-remover',
    name: '去重工具',
    description: '文本行去重，保留唯一行',
    category: 'utility',
  },
  {
    slug: 'text-stats',
    name: '文本统计',
    description: '词频统计与文本分析',
    category: 'utility',
  },
  {
    slug: 'escape-html',
    name: 'HTML 转义',
    description: 'HTML 内容安全转义',
    category: 'utility',
  },
  {
    slug: 'chmod-calc',
    name: 'chmod 计算',
    description: 'Unix 文件权限可视化计算',
    category: 'utility',
  },
  {
    slug: 'sort-json',
    name: 'JSON 键排序',
    description: 'JSON 对象键按字母序排序',
    category: 'utility',
  },
]

export const PROBE_TOOLS = ALL_TOOLS.filter(t => t.category === 'probe')
export const UTILITY_TOOLS = ALL_TOOLS.filter(t => t.category === 'utility')

export function getToolBySlug(slug: string): ToolDef | undefined {
  return ALL_TOOLS.find(t => t.slug === slug)
}
