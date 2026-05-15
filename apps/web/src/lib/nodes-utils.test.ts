import { describe, it, expect } from "vitest"
import {
  mapStatus,
  formatIP,
  aggregateStats,
  filterNodes,
  type NodeEntry,
} from "./nodes-utils"

const sampleNodes: NodeEntry[] = [
  { id: "n1", asn: "AS4134", carrier: "中国电信", region: "北京", exitIp: "1.2.3.4", status: "online", country: "CN" },
  { id: "n2", asn: "AS4837", carrier: "中国联通", region: "上海", exitIp: "2.3.4.5", status: "online", country: "CN" },
  { id: "n3", asn: "AS9269", carrier: "PCCW", region: "香港", exitIp: "3.4.5.6", status: "offline", country: "HK" },
  { id: "n4", asn: "AS2497", carrier: "IIJ", region: "东京", exitIp: "4.5.6.7", status: "degraded", country: "JP" },
  { id: "n5", asn: "AS16509", carrier: "AWS", region: "弗吉尼亚", exitIp: "5.6.7.8", status: "online", country: "US" },
]

describe("mapStatus", () => {
  it("maps online to success variant", () => {
    const result = mapStatus("online")
    expect(result.label).toBe("在线")
    expect(result.variant).toBe("success")
  })

  it("maps offline to destructive variant", () => {
    const result = mapStatus("offline")
    expect(result.label).toBe("离线")
    expect(result.variant).toBe("destructive")
  })

  it("maps degraded to warning variant", () => {
    const result = mapStatus("degraded")
    expect(result.label).toBe("降级")
    expect(result.variant).toBe("warning")
  })

  it("returns secondary variant for unknown status", () => {
    const result = mapStatus("unknown")
    expect(result.label).toBe("unknown")
    expect(result.variant).toBe("secondary")
  })
})

describe("formatIP", () => {
  it("returns IPv4 unchanged", () => {
    expect(formatIP("192.168.1.1")).toBe("192.168.1.1")
  })

  it("wraps bare IPv6 in brackets", () => {
    expect(formatIP("2001:db8::1")).toBe("[2001:db8::1]")
  })

  it("leaves already-bracketed IPv6 unchanged", () => {
    expect(formatIP("[2001:db8::1]")).toBe("[2001:db8::1]")
  })

  it("returns empty string for empty input", () => {
    expect(formatIP("")).toBe("")
  })

  it("handles full IPv6 address", () => {
    expect(formatIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")).toBe(
      "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]"
    )
  })
})

describe("aggregateStats", () => {
  it("counts total nodes correctly", () => {
    expect(aggregateStats(sampleNodes).total).toBe(5)
  })

  it("counts only online nodes", () => {
    expect(aggregateStats(sampleNodes).online).toBe(3)
  })

  it("counts unique countries", () => {
    expect(aggregateStats(sampleNodes).countries).toBe(4)
  })

  it("counts unique carriers", () => {
    expect(aggregateStats(sampleNodes).carriers).toBe(5)
  })

  it("returns zeros for empty array", () => {
    const stats = aggregateStats([])
    expect(stats.total).toBe(0)
    expect(stats.online).toBe(0)
    expect(stats.countries).toBe(0)
    expect(stats.carriers).toBe(0)
  })

  it("handles all offline nodes", () => {
    const offlineNodes = sampleNodes.map((n) => ({ ...n, status: "offline" as const }))
    expect(aggregateStats(offlineNodes).online).toBe(0)
  })
})

describe("filterNodes", () => {
  it("returns all nodes when filters are empty", () => {
    expect(filterNodes(sampleNodes, {})).toHaveLength(5)
  })

  it("filters by country", () => {
    const result = filterNodes(sampleNodes, { country: "CN" })
    expect(result).toHaveLength(2)
    expect(result.every((n) => n.country === "CN")).toBe(true)
  })

  it("returns all when country is 'all'", () => {
    expect(filterNodes(sampleNodes, { country: "all" })).toHaveLength(5)
  })

  it("filters by carrier", () => {
    const result = filterNodes(sampleNodes, { carrier: "IIJ" })
    expect(result).toHaveLength(1)
    expect(result[0]!.id).toBe("n4")
  })

  it("returns all when carrier is 'all'", () => {
    expect(filterNodes(sampleNodes, { carrier: "all" })).toHaveLength(5)
  })

  it("filters by status", () => {
    const result = filterNodes(sampleNodes, { status: "online" })
    expect(result).toHaveLength(3)
    expect(result.every((n) => n.status === "online")).toBe(true)
  })

  it("filters by offline status", () => {
    const result = filterNodes(sampleNodes, { status: "offline" })
    expect(result).toHaveLength(1)
    expect(result[0]!.id).toBe("n3")
  })

  it("searches by region", () => {
    const result = filterNodes(sampleNodes, { search: "东京" })
    expect(result).toHaveLength(1)
    expect(result[0]!.id).toBe("n4")
  })

  it("searches by ASN (case-insensitive)", () => {
    const result = filterNodes(sampleNodes, { search: "as4134" })
    expect(result).toHaveLength(1)
    expect(result[0]!.id).toBe("n1")
  })

  it("searches by exit IP", () => {
    const result = filterNodes(sampleNodes, { search: "5.6.7.8" })
    expect(result).toHaveLength(1)
    expect(result[0]!.id).toBe("n5")
  })

  it("combines country and status filters", () => {
    const result = filterNodes(sampleNodes, { country: "CN", status: "online" })
    expect(result).toHaveLength(2)
  })

  it("returns empty when no nodes match", () => {
    const result = filterNodes(sampleNodes, { country: "DE" })
    expect(result).toHaveLength(0)
  })
})
