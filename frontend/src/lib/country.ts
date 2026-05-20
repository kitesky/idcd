/**
 * 将 ISO 3166-1 alpha-2 国家代码转换为国旗 emoji
 * @param code 两字母国家代码（如 "CN", "US"）
 * @returns 国旗 emoji 或地球 emoji（如果代码无效）
 */
export function countryFlag(code: string): string {
  if (!code || code.length !== 2) return "🌐"
  return String.fromCodePoint(
    ...code.toUpperCase().split("").map(c => 0x1F1E6 + c.charCodeAt(0) - 65)
  )
}

/**
 * 国家代码到地理坐标的映射（用于地图展示）
 * 坐标格式：[经度, 纬度]
 */
export const countryCoords: Record<string, [number, number]> = {
  CN: [104.1954, 35.8617],   // 中国
  HK: [114.1694, 22.3193],   // 香港
  TW: [120.9605, 23.6978],   // 台湾
  JP: [138.2529, 36.2048],   // 日本
  KR: [127.7669, 35.9078],   // 韩国
  SG: [103.8198, 1.3521],    // 新加坡
  US: [-95.7129, 37.0902],   // 美国
  GB: [-3.4360, 55.3781],    // 英国
  DE: [10.4515, 51.1657],    // 德国
  FR: [2.2137, 46.2276],     // 法国
  AU: [133.7751, -25.2744],  // 澳大利亚
  CA: [-106.3468, 56.1304],  // 加拿大
  IN: [78.9629, 20.5937],    // 印度
  BR: [-51.9253, -14.2350],  // 巴西
  RU: [105.3188, 61.5240],   // 俄罗斯
  NL: [5.2913, 52.1326],     // 荷兰
  SE: [18.6435, 60.1282],    // 瑞典
  NO: [8.4689, 60.4720],     // 挪威
  FI: [25.7482, 61.9241],    // 芬兰
  IT: [12.5674, 41.8719],    // 意大利
  ES: [-3.7492, 40.4637],    // 西班牙
  CH: [8.2275, 46.8182],     // 瑞士
}

/**
 * 获取国家代码的中文名称
 */
export function getCountryName(code: string): string {
  const names: Record<string, string> = {
    CN: "中国大陆",
    HK: "中国香港",
    TW: "中国台湾",
    JP: "日本",
    KR: "韩国",
    SG: "新加坡",
    US: "美国",
    GB: "英国",
    DE: "德国",
    FR: "法国",
    AU: "澳大利亚",
    CA: "加拿大",
    IN: "印度",
    BR: "巴西",
    RU: "俄罗斯",
    NL: "荷兰",
    SE: "瑞典",
    NO: "挪威",
    FI: "芬兰",
    IT: "意大利",
    ES: "西班牙",
    CH: "瑞士",
  }
  return names[code] || code
}
