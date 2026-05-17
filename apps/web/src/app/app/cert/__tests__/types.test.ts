import { describe, expect, it } from "vitest"
import {
  isExpiringSoon,
  isIdn,
  isWildcard,
  parseSanInput,
  toPunycode,
} from "../types"

describe("parseSanInput", () => {
  it("splits on newlines, commas, semicolons and whitespace", () => {
    expect(
      parseSanInput("a.example.com\nb.example.com, c.example.com; d.example.com e.example.com"),
    ).toEqual([
      "a.example.com",
      "b.example.com",
      "c.example.com",
      "d.example.com",
      "e.example.com",
    ])
  })

  it("trims and lowercases entries", () => {
    expect(parseSanInput("  Example.COM ,  WWW.example.com ")).toEqual([
      "example.com",
      "www.example.com",
    ])
  })

  it("de-duplicates while preserving order", () => {
    expect(parseSanInput("a.com,b.com,a.com,B.com")).toEqual(["a.com", "b.com"])
  })

  it("returns an empty array for empty / whitespace input", () => {
    expect(parseSanInput("")).toEqual([])
    expect(parseSanInput("   \n\t")).toEqual([])
  })

  it("preserves wildcard prefixes", () => {
    expect(parseSanInput("*.example.com")).toEqual(["*.example.com"])
  })
})

describe("isWildcard", () => {
  it("detects leading *. prefix only", () => {
    expect(isWildcard("*.example.com")).toBe(true)
    expect(isWildcard("example.com")).toBe(false)
    expect(isWildcard("foo.*.example.com")).toBe(false)
  })
})

describe("isIdn", () => {
  it("returns false for pure ASCII hosts", () => {
    expect(isIdn("example.com")).toBe(false)
    expect(isIdn("*.example.com")).toBe(false)
  })

  it("returns true for non-ASCII hosts", () => {
    expect(isIdn("中文.com")).toBe(true)
    expect(isIdn("*.中文.com")).toBe(true)
  })
})

describe("toPunycode", () => {
  it("returns ASCII hosts unchanged", () => {
    expect(toPunycode("example.com")).toBe("example.com")
  })

  it("encodes a non-ASCII host via URL parser", () => {
    const out = toPunycode("中文.com")
    expect(out.startsWith("xn--")).toBe(true)
  })

  it("preserves wildcard prefix on IDN host", () => {
    const out = toPunycode("*.中文.com")
    expect(out.startsWith("*.xn--")).toBe(true)
  })
})

describe("isExpiringSoon", () => {
  it("returns true when expiry is within the threshold and in the future", () => {
    const inTwoWeeks = new Date(Date.now() + 14 * 86400_000).toISOString()
    expect(isExpiringSoon(inTwoWeeks)).toBe(true)
  })

  it("returns false when expiry is far away", () => {
    const inSixtyDays = new Date(Date.now() + 60 * 86400_000).toISOString()
    expect(isExpiringSoon(inSixtyDays)).toBe(false)
  })

  it("returns false when already expired", () => {
    const yesterday = new Date(Date.now() - 86400_000).toISOString()
    expect(isExpiringSoon(yesterday)).toBe(false)
  })

  it("returns false for invalid input", () => {
    expect(isExpiringSoon("not-a-date")).toBe(false)
  })
})
