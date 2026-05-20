package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-pdf/fpdf"
)

// renderPDF produces a branded evidence report PDF for one Verdict
// order. The output must remain a valid PDF that lib/attest/pdfsign can
// add a PAdES-T signature to (CMS ByteRange placeholder injected later)
// and that github.com/digitorus/pdf can parse for the /verify endpoint.
//
// CJK fallback strategy: we try to load a system CJK TTF from
// $IDCD_PDF_CJK_FONT_PATH (default /usr/share/fonts/truetype/wqy/wqy-zenhei.ttc).
// If the font is missing or fpdf rejects it (TTC collections, corrupt
// file), we silently degrade to built-in Helvetica with English-only
// labels — a degraded PDF is still a valid PAdES input and the
// downstream verifier doesn't care about glyphs.
func renderPDF(o *Order, obs []observation, nodes []string) ([]byte, error) {
	if o == nil {
		return nil, fmt.Errorf("renderPDF: nil order")
	}

	tpl := templateMeta(o.Template)
	cjk := loadCJKFont()

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 25)
	pdf.SetCreator("idcd", true)
	pdf.SetAuthor("idcd Evidence Service", true)
	pdf.SetTitle(tpl.titleEN+" - "+o.ID, true)
	pdf.AliasNbPages("")

	bodyFont := "Helvetica"
	if cjk.ok {
		pdf.AddUTF8FontFromBytes("idcdCJK", "", cjk.bytes)
		if pdf.Err() {
			pdf.ClearError()
			cjk.ok = false
		} else {
			bodyFont = "idcdCJK"
		}
	}

	pdf.SetFooterFunc(func() {
		pdf.SetY(-20)
		pdf.SetFont(bodyFont, "", 8)
		for _, line := range disclaimerLines(cjk.ok) {
			pdf.CellFormat(0, 4, line, "", 1, "C", false, 0, "")
		}
		pdf.CellFormat(0, 4, fmt.Sprintf("Page %d / {nb}", pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	pdf.AddPage()
	writeCover(pdf, bodyFont, cjk.ok, tpl, o)

	pdf.AddPage()
	writeNodeMap(pdf, bodyFont, cjk.ok, obs, nodes)
	writeTimeSeries(pdf, bodyFont, cjk.ok, obs)
	writeSummary(pdf, bodyFont, cjk.ok, obs)

	pdf.AddPage()
	writeLegalSection(pdf, bodyFont, cjk.ok)

	if pdf.Err() {
		return nil, fmt.Errorf("renderPDF: %w", pdf.Error())
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("renderPDF: output: %w", err)
	}
	return buf.Bytes(), nil
}

// -----------------------------------------------------------------------
// template metadata
// -----------------------------------------------------------------------

type templateInfo struct {
	key     string
	titleCN string
	titleEN string
	leadCN  string
	leadEN  string
}

var templateLabels = map[string]templateInfo{
	"sla":        {"sla", "SLA 观测报告", "SLA Observation Report", "重点强调可用性时序与一致性", "Focus on availability time series and consistency"},
	"incident":   {"incident", "事件取证报告", "Incident Evidence Report", "重点强调时间序列与节点跨证", "Focus on time series and cross-node corroboration"},
	"compliance": {"compliance", "合规观测报告", "Compliance Observation Report", "重点强调节点覆盖与汇总", "Focus on node coverage and summary"},
	"legal":      {"legal", "法律辅助报告", "Legal Auxiliary Report", "重点强调一手观测与法律边界", "Focus on first-hand observation and legal boundary"},
}

func templateMeta(key string) templateInfo {
	if t, ok := templateLabels[strings.ToLower(strings.TrimSpace(key))]; ok {
		return t
	}
	return templateInfo{
		key:     "unknown",
		titleCN: "观测证据报告",
		titleEN: "Evidence Observation Report",
		leadCN:  "通用观测数据",
		leadEN:  "Generic observation data",
	}
}

// -----------------------------------------------------------------------
// disclaimer / legal boundary text
// -----------------------------------------------------------------------

// disclaimerLines returns the legal-boundary block. With CJK font we
// emit the spec-mandated Chinese text; without it we emit the English
// translation (Helvetica/WinAnsi cannot encode CJK glyphs).
func disclaimerLines(cjk bool) []string {
	if cjk {
		return []string{
			"本报告为 idcd 提供的一手观测数据,不构成司法鉴定结论。",
			"Report Type: observation_only",
			"第三方使用本报告时,应基于 report_type 字段决定使用方式。",
		}
	}
	return []string{
		"This report is first-hand observation data provided by idcd; it is not a judicial conclusion.",
		"Report Type: observation_only",
		"Third parties must base any use of this report on the report_type field.",
	}
}

// -----------------------------------------------------------------------
// CJK font loader
// -----------------------------------------------------------------------

const defaultCJKFontPath = "/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc"

type cjkFont struct {
	ok    bool
	bytes []byte
}

var (
	cjkOnce sync.Once
	cjkVal  cjkFont
)

func loadCJKFont() cjkFont {
	cjkOnce.Do(func() {
		path := os.Getenv("IDCD_PDF_CJK_FONT_PATH")
		if path == "" {
			path = defaultCJKFontPath
		}
		b, err := os.ReadFile(path)
		if err != nil {
			cjkVal = cjkFont{ok: false}
			return
		}
		// TTC collections are not single-face TTFs; gofpdf's parser
		// only handles standalone TTF. Refuse collections so we
		// degrade cleanly instead of producing a corrupt PDF.
		if len(b) >= 4 && string(b[:4]) == "ttcf" {
			cjkVal = cjkFont{ok: false}
			return
		}
		cjkVal = cjkFont{ok: true, bytes: b}
	})
	return cjkVal
}

// resetCJKFontForTest lets tests in this package re-evaluate the env
// var after mutating it.
func resetCJKFontForTest() {
	cjkOnce = sync.Once{}
	cjkVal = cjkFont{}
}

// -----------------------------------------------------------------------
// section writers
// -----------------------------------------------------------------------

func writeCover(pdf *fpdf.Fpdf, font string, cjk bool, tpl templateInfo, o *Order) {
	pdf.SetFont(font, "", 22)
	pdf.CellFormat(0, 14, "idcd", "", 1, "L", false, 0, "")
	pdf.SetFont(font, "", 16)
	title := tpl.titleEN
	if cjk {
		title = tpl.titleCN
	}
	pdf.CellFormat(0, 10, title, "", 1, "L", false, 0, "")

	pdf.SetFont(font, "", 10)
	subtitle := tpl.leadEN
	if cjk {
		subtitle = tpl.leadCN
	}
	pdf.CellFormat(0, 6, subtitle, "", 1, "L", false, 0, "")
	pdf.Ln(6)

	pdf.SetFont(font, "", 11)
	rows := [][2]string{
		{label(cjk, "订单 ID", "Order ID"), o.ID},
		{label(cjk, "所有者", "Owner"), ownerFingerprint(o.OwnerID)},
		{label(cjk, "目标", "Target"), o.Target},
		{label(cjk, "时间窗口起", "Window Start (UTC)"), formatTime(o.TimeWindowStart)},
		{label(cjk, "时间窗口止", "Window End (UTC)"), formatTime(o.TimeWindowEnd)},
		{label(cjk, "模板", "Template"), tpl.key},
		{label(cjk, "报告类型", "Report Type"), "observation_only"},
	}
	for _, row := range rows {
		pdf.CellFormat(55, 7, row[0], "1", 0, "L", false, 0, "")
		pdf.CellFormat(0, 7, row[1], "1", 1, "L", false, 0, "")
	}
}

func writeNodeMap(pdf *fpdf.Fpdf, font string, cjk bool, obs []observation, nodes []string) {
	pdf.SetFont(font, "", 14)
	pdf.CellFormat(0, 10, label(cjk, "1. 节点分布", "1. Node Map"), "", 1, "L", false, 0, "")

	counts := map[string]int{}
	okCounts := map[string]int{}
	for _, o := range obs {
		counts[o.NodeID]++
		if o.OK {
			okCounts[o.NodeID]++
		}
	}
	allNodes := mergeNodes(nodes, counts)

	pdf.SetFont(font, "", 11)
	pdf.CellFormat(70, 7, label(cjk, "节点 ID", "Node ID"), "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 7, label(cjk, "观测次数", "Observations"), "1", 0, "C", false, 0, "")
	pdf.CellFormat(0, 7, label(cjk, "一致性 %", "Consistency %"), "1", 1, "C", false, 0, "")

	for _, n := range allNodes {
		total := counts[n]
		var pct float64
		if total > 0 {
			pct = float64(okCounts[n]) / float64(total) * 100
		}
		pdf.CellFormat(70, 6, n, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, fmt.Sprintf("%d", total), "1", 0, "C", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("%.1f", pct), "1", 1, "C", false, 0, "")
	}
	pdf.Ln(4)
}

func writeTimeSeries(pdf *fpdf.Fpdf, font string, cjk bool, obs []observation) {
	pdf.SetFont(font, "", 14)
	pdf.CellFormat(0, 10, label(cjk, "2. 时间序列", "2. Time Series"), "", 1, "L", false, 0, "")

	sorted := make([]observation, len(obs))
	copy(sorted, obs)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	pdf.SetFont(font, "", 11)
	pdf.CellFormat(55, 7, label(cjk, "时间 (UTC)", "Timestamp (UTC)"), "1", 0, "L", false, 0, "")
	pdf.CellFormat(55, 7, label(cjk, "节点", "Node"), "1", 0, "L", false, 0, "")
	pdf.CellFormat(25, 7, label(cjk, "状态", "OK"), "1", 0, "C", false, 0, "")
	pdf.CellFormat(0, 7, label(cjk, "延迟 (ms)", "Latency (ms)"), "1", 1, "C", false, 0, "")

	for _, o := range sorted {
		ok := "false"
		if o.OK {
			ok = "true"
		}
		pdf.CellFormat(55, 6, o.Timestamp.UTC().Format("2006-01-02 15:04:05"), "1", 0, "L", false, 0, "")
		pdf.CellFormat(55, 6, o.NodeID, "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 6, ok, "1", 0, "C", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("%d", o.Latency.Milliseconds()), "1", 1, "C", false, 0, "")
	}
	pdf.Ln(4)
}

func writeSummary(pdf *fpdf.Fpdf, font string, cjk bool, obs []observation) {
	pdf.SetFont(font, "", 14)
	pdf.CellFormat(0, 10, label(cjk, "3. 汇总", "3. Summary"), "", 1, "L", false, 0, "")

	total := len(obs)
	okCount := 0
	for _, o := range obs {
		if o.OK {
			okCount++
		}
	}
	failCount := total - okCount
	var consistency float64
	if total > 0 {
		consistency = float64(okCount) / float64(total) * 100
	}
	p50, p95 := percentilesMs(obs)

	pdf.SetFont(font, "", 11)
	rows := [][2]string{
		{label(cjk, "总观测数", "Total observations"), fmt.Sprintf("%d", total)},
		{label(cjk, "OK 计数", "OK count"), fmt.Sprintf("%d", okCount)},
		{label(cjk, "失败计数", "Fail count"), fmt.Sprintf("%d", failCount)},
		{label(cjk, "一致性 %", "Consistency %"), fmt.Sprintf("%.2f", consistency)},
		{label(cjk, "p50 延迟 (ms)", "p50 latency (ms)"), fmt.Sprintf("%d", p50)},
		{label(cjk, "p95 延迟 (ms)", "p95 latency (ms)"), fmt.Sprintf("%d", p95)},
	}
	for _, r := range rows {
		pdf.CellFormat(70, 7, r[0], "1", 0, "L", false, 0, "")
		pdf.CellFormat(0, 7, r[1], "1", 1, "L", false, 0, "")
	}
}

func writeLegalSection(pdf *fpdf.Fpdf, font string, cjk bool) {
	pdf.SetFont(font, "", 14)
	pdf.CellFormat(0, 10, label(cjk, "4. 法律边界声明", "4. Legal Boundary Disclaimer"), "", 1, "L", false, 0, "")

	pdf.SetFont(font, "", 11)
	for _, line := range disclaimerLines(cjk) {
		pdf.MultiCell(0, 7, line, "", "L", false)
	}
}

// -----------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------

func label(cjk bool, zh, en string) string {
	if cjk {
		return zh
	}
	return en
}

// ownerFingerprint redacts a raw owner ID to a 12-hex-char SHA-256
// prefix so the PDF never leaks the underlying user ID in plain text.
func ownerFingerprint(ownerID string) string {
	if ownerID == "" {
		return "anon"
	}
	sum := sha256.Sum256([]byte(ownerID))
	return "fp_" + hex.EncodeToString(sum[:6])
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}

// mergeNodes returns the union of the explicit nodes slice and any node
// IDs observed in obs, preserving the explicit ordering first.
func mergeNodes(explicit []string, counts map[string]int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(explicit)+len(counts))
	for _, n := range explicit {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	extras := make([]string, 0)
	for n := range counts {
		if _, ok := seen[n]; ok {
			continue
		}
		extras = append(extras, n)
	}
	sort.Strings(extras)
	out = append(out, extras...)
	return out
}

// percentilesMs computes p50 / p95 latency in milliseconds. Uses the
// nearest-rank method (ceil(p * n)) so tiny samples behave intuitively.
func percentilesMs(obs []observation) (p50, p95 int64) {
	if len(obs) == 0 {
		return 0, 0
	}
	xs := make([]int64, len(obs))
	for i, o := range obs {
		xs[i] = o.Latency.Milliseconds()
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	return nearestRank(xs, 0.5), nearestRank(xs, 0.95)
}

func nearestRank(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(float64(len(sorted))*p + 0.999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
