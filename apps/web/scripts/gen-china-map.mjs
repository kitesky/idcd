/**
 * 预生成中国省份 SVG 路径
 * 关键：直接逐点投影（跳过 geoPath 球面弧插值，避免路径溢出视口）
 * 运行：node scripts/gen-china-map.mjs
 * 输出：public/china-map-data.json
 */
import { readFileSync, writeFileSync } from "fs"
import { geoMercator } from "d3-geo"

const W = 640, H = 440

// 中国大陆 Mercator 投影：手动设 center/scale，不用 fitExtent（会被 DataV 极坐标干扰）
const projection = geoMercator()
  .center([104, 37])
  .scale(570)
  .translate([W / 2, H / 2])

// 直接将坐标数组转为 SVG path d 属性（直线连接，不用球面弧插值）
function ringToPath(ring) {
  const pts = ring
    .map(([lng, lat]) => {
      const p = projection([lng, lat])
      return p ? `${p[0].toFixed(2)},${p[1].toFixed(2)}` : null
    })
    .filter(Boolean)
  if (pts.length < 3) return ""
  return `M${pts.join("L")}Z`
}

function featureToPath(feature) {
  const { type, coordinates } = feature.geometry
  if (type === "Polygon") {
    // 只用外环（coordinates[0]），忽略洞
    return ringToPath(coordinates[0])
  }
  if (type === "MultiPolygon") {
    // 取最大的子多边形（按坐标数判断）
    const largest = coordinates.reduce((a, b) => a[0].length > b[0].length ? a : b)
    return ringToPath(largest[0])
  }
  return ""
}

const geoJson = JSON.parse(readFileSync("public/china-provinces.json", "utf8"))

// 过滤：只保留 Polygon/MultiPolygon，且中心纬度 > 17°（排除南海极南离岛）
const features = geoJson.features.filter(f => {
  const t = f.geometry?.type
  if (t !== "Polygon" && t !== "MultiPolygon") return false
  const center = f.properties?.center
  const lat = center?.[1] ?? 90
  return lat > 17
})

const output = features.map(f => {
  const center = f.properties?.center
  const centroid = f.properties?.centroid
  return {
    id:       f.properties?.adcode,
    name:     f.properties?.name,
    center:   center   ? projection(center).map(v => Math.round(v * 10) / 10)   : null,
    centroid: centroid ? projection(centroid).map(v => Math.round(v * 10) / 10) : null,
    d:        featureToPath(f),
  }
}).filter(p => p.d) // 排除空路径

writeFileSync(
  "public/china-map-data.json",
  JSON.stringify({ w: W, h: H, provinces: output })
)
console.log(`✓ Generated ${output.length} provinces → public/china-map-data.json`)

// 验证几个关键省份的坐标范围
const testNames = ["新疆维吾尔自治区", "云南省", "北京市", "黑龙江省"]
for (const name of testNames) {
  const p = output.find(o => o.name === name)
  if (p) console.log(`  ${p.name}: center=${JSON.stringify(p.center)}`)
}
