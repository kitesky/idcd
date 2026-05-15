import { describe, it, expect } from "vitest"
import {
  CDN_DATA,
  REGION_LATENCY_DATA,
  ISP_AVAILABILITY_DATA,
  NODE_COUNT,
  getCurrentMonthLabel,
  getLatencyVariant,
} from "../leaderboard-data"

describe("leaderboard-data — CDN entries", () => {
  it("CDN 数据应包含 10 条记录", () => {
    expect(CDN_DATA).toHaveLength(10)
  })

  it("CDN 数据按 globalP50 升序排列后，首项最小", () => {
    const sorted = [...CDN_DATA].sort((a, b) => a.globalP50 - b.globalP50)
    expect(sorted[0]!.globalP50).toBeLessThanOrEqual(sorted[1]!.globalP50)
  })

  it("CDN 数据按 globalP50 升序排列后，末项最大", () => {
    const sorted = [...CDN_DATA].sort((a, b) => a.globalP50 - b.globalP50)
    const last = sorted.length - 1
    expect(sorted[last]!.globalP50).toBeGreaterThanOrEqual(sorted[last - 1]!.globalP50)
  })

  it("所有 CDN 条目 rank > 0、非空 name、trend 长度为 7", () => {
    for (const cdn of CDN_DATA) {
      expect(cdn.rank).toBeGreaterThan(0)
      expect(cdn.name).toBeTruthy()
      expect(cdn.trend).toHaveLength(7)
    }
  })

  it("所有 CDN globalP50 均为正整数（单位 ms）", () => {
    for (const cdn of CDN_DATA) {
      expect(cdn.globalP50).toBeGreaterThan(0)
      expect(Number.isInteger(cdn.globalP50)).toBe(true)
    }
  })
})

describe("leaderboard-data — Region entries", () => {
  it("区域数据应包含 6 个大陆", () => {
    expect(REGION_LATENCY_DATA).toHaveLength(6)
  })

  it("每个大陆应有 5 个国家/地区", () => {
    for (const region of REGION_LATENCY_DATA) {
      expect(region.countries).toHaveLength(5)
    }
  })

  it("亚洲大陆存在且包含香港", () => {
    const asia = REGION_LATENCY_DATA.find((r) => r.continent === "亚洲")
    expect(asia).toBeDefined()
    const hk = asia?.countries.find((c) => c.name === "香港")
    expect(hk).toBeDefined()
  })
})

describe("leaderboard-data — ISP Availability entries", () => {
  it("ISP 数据应包含至少 10 条记录", () => {
    expect(ISP_AVAILABILITY_DATA.length).toBeGreaterThanOrEqual(10)
  })

  it("所有 ISP 可用率在 0-100 之间", () => {
    for (const isp of ISP_AVAILABILITY_DATA) {
      expect(isp.availability30d).toBeGreaterThan(0)
      expect(isp.availability30d).toBeLessThanOrEqual(100)
    }
  })

  it("ISP 按 rank 升序排列", () => {
    for (let i = 1; i < ISP_AVAILABILITY_DATA.length; i++) {
      expect(ISP_AVAILABILITY_DATA[i]!.rank).toBeGreaterThan(
        ISP_AVAILABILITY_DATA[i - 1]!.rank
      )
    }
  })
})

describe("leaderboard-data — helpers", () => {
  it("getLatencyVariant — < 50ms 返回 success", () => {
    expect(getLatencyVariant(18)).toBe("success")
    expect(getLatencyVariant(49)).toBe("success")
  })

  it("getLatencyVariant — 50ms 返回 warning（边界）", () => {
    expect(getLatencyVariant(50)).toBe("warning")
  })

  it("getLatencyVariant — 200ms 返回 warning（边界）", () => {
    expect(getLatencyVariant(200)).toBe("warning")
  })

  it("getLatencyVariant — 201ms 返回 destructive", () => {
    expect(getLatencyVariant(201)).toBe("destructive")
    expect(getLatencyVariant(500)).toBe("destructive")
  })

  it("NODE_COUNT 应大于 0", () => {
    expect(NODE_COUNT).toBeGreaterThan(0)
  })

  it("getCurrentMonthLabel 应包含当前年份", () => {
    const label = getCurrentMonthLabel()
    const year = new Date().getFullYear().toString()
    expect(label).toContain(year)
  })

  it('getCurrentMonthLabel 应包含 年 和 月 字', () => {
    const label = getCurrentMonthLabel()
    expect(label).toContain("年")
    expect(label).toContain("月")
  })
})
