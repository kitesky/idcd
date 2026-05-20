import { describe, it, expect } from "vitest"
import { countryFlag, getCountryName, countryCoords } from "../country"

describe("countryFlag", () => {
  it("应该返回正确的国旗 emoji", () => {
    expect(countryFlag("CN")).toBe("🇨🇳")
    expect(countryFlag("US")).toBe("🇺🇸")
    expect(countryFlag("JP")).toBe("🇯🇵")
    expect(countryFlag("HK")).toBe("🇭🇰")
  })

  it("应该处理小写输入", () => {
    expect(countryFlag("cn")).toBe("🇨🇳")
    expect(countryFlag("us")).toBe("🇺🇸")
  })

  it("应该处理无效输入", () => {
    expect(countryFlag("")).toBe("🌐")
    expect(countryFlag("X")).toBe("🌐")
    expect(countryFlag("XXX")).toBe("🌐")
  })
})

describe("getCountryName", () => {
  it("应该返回正确的国家名称", () => {
    expect(getCountryName("CN")).toBe("中国大陆")
    expect(getCountryName("HK")).toBe("中国香港")
    expect(getCountryName("US")).toBe("美国")
    expect(getCountryName("JP")).toBe("日本")
  })

  it("应该处理未知国家代码", () => {
    expect(getCountryName("XX")).toBe("XX")
    expect(getCountryName("ZZ")).toBe("ZZ")
  })
})

describe("countryCoords", () => {
  it("应该包含常见国家的坐标", () => {
    expect(countryCoords["CN"]).toBeDefined()
    expect(countryCoords["US"]).toBeDefined()
    expect(countryCoords["JP"]).toBeDefined()
    expect(countryCoords["HK"]).toBeDefined()
  })

  it("坐标格式应该正确", () => {
    const [lng, lat] = countryCoords["CN"]!
    expect(typeof lng).toBe("number")
    expect(typeof lat).toBe("number")
    expect(lng).toBeGreaterThan(-180)
    expect(lng).toBeLessThan(180)
    expect(lat).toBeGreaterThan(-90)
    expect(lat).toBeLessThan(90)
  })
})
