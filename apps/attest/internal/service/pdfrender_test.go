package service

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	pdfreader "github.com/digitorus/pdf"
)

func fixedOrder(tpl string) *Order {
	return &Order{
		ID:              "vo_test_001",
		OwnerID:         "user_42",
		Template:        tpl,
		Target:          "https://example.com/health",
		TimeWindowStart: time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC),
		TimeWindowEnd:   time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC),
		Status:          "paid",
	}
}

func fixedObs() []observation {
	t0 := time.Date(2026, 5, 17, 9, 30, 0, 0, time.UTC)
	return []observation{
		{NodeID: "node-cn-bj", Timestamp: t0.Add(2 * time.Second), Latency: 42 * time.Millisecond, OK: true},
		{NodeID: "node-cn-sh", Timestamp: t0, Latency: 51 * time.Millisecond, OK: true},
		{NodeID: "node-cn-gz", Timestamp: t0.Add(time.Second), Latency: 47 * time.Millisecond, OK: false},
	}
}

func TestRenderPDF_HasMagicHeader(t *testing.T) {
	out, err := renderPDF(&Order{ID: "vo_x"}, nil, nil)
	if err != nil {
		t.Fatalf("renderPDF: %v", err)
	}
	if len(out) < 5 || string(out[:5]) != "%PDF-" {
		t.Fatalf("renderPDF output missing %%PDF- header: %q", out[:minInt(5, len(out))])
	}
}

func TestRenderPDF_ParsesAndContainsDisclaimer(t *testing.T) {
	resetCJKFontForTest()
	t.Setenv("IDCD_PDF_CJK_FONT_PATH", "/nonexistent/font.ttf")
	defer resetCJKFontForTest()

	out, err := renderPDF(fixedOrder("sla"), fixedObs(), []string{"node-cn-bj", "node-cn-sh", "node-cn-gz"})
	if err != nil {
		t.Fatalf("renderPDF: %v", err)
	}

	r, err := pdfreader.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("pdf.NewReader: %v", err)
	}
	if n := r.NumPage(); n < 1 {
		t.Fatalf("expected at least 1 page, got %d", n)
	}

	// digitorus/pdf drops spaces from extracted Text records, so
	// substring assertions look for the compact (no-space) form.
	text := extractAllText(t, r)
	for _, want := range []string{
		"first-handobservationdataprovidedbyidcd",
		"observation_only",
		"vo_test_001",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected substring %q not in extracted text (len=%d)", want, len(text))
		}
	}
}

func TestRenderPDF_AllFourTemplatesRender(t *testing.T) {
	resetCJKFontForTest()
	t.Setenv("IDCD_PDF_CJK_FONT_PATH", "/nonexistent/font.ttf")
	defer resetCJKFontForTest()

	for _, tpl := range []string{"sla", "incident", "compliance", "legal"} {
		t.Run(tpl, func(t *testing.T) {
			out, err := renderPDF(fixedOrder(tpl), fixedObs(), nil)
			if err != nil {
				t.Fatalf("renderPDF(%s): %v", tpl, err)
			}
			if string(out[:5]) != "%PDF-" {
				t.Fatalf("missing magic header for %s", tpl)
			}
			r, err := pdfreader.NewReader(bytes.NewReader(out), int64(len(out)))
			if err != nil {
				t.Fatalf("pdf.NewReader(%s): %v", tpl, err)
			}
			if r.NumPage() < 1 {
				t.Fatalf("template %s produced 0 pages", tpl)
			}
		})
	}
}

func TestRenderPDF_NilOrder(t *testing.T) {
	if _, err := renderPDF(nil, nil, nil); err == nil {
		t.Fatalf("expected error for nil order")
	}
}

func TestRenderPDF_EmptyObservations(t *testing.T) {
	resetCJKFontForTest()
	t.Setenv("IDCD_PDF_CJK_FONT_PATH", "/nonexistent/font.ttf")
	defer resetCJKFontForTest()

	out, err := renderPDF(fixedOrder("sla"), nil, nil)
	if err != nil {
		t.Fatalf("renderPDF: %v", err)
	}
	r, err := pdfreader.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("pdf.NewReader: %v", err)
	}
	if r.NumPage() < 3 {
		t.Fatalf("want at least 3 pages (cover/sections/legal), got %d", r.NumPage())
	}
}

func TestTemplateMeta_FallbackForUnknown(t *testing.T) {
	tpl := templateMeta("not-a-real-template")
	if tpl.key != "unknown" {
		t.Fatalf("want key=unknown, got %q", tpl.key)
	}
	if tpl.titleEN == "" {
		t.Fatalf("titleEN should be populated for fallback")
	}
}

func TestTemplateMeta_NormalisesWhitespaceAndCase(t *testing.T) {
	tpl := templateMeta("  SLA  ")
	if tpl.key != "sla" {
		t.Fatalf("want key=sla, got %q", tpl.key)
	}
}

func TestOwnerFingerprint(t *testing.T) {
	if fp := ownerFingerprint(""); fp != "anon" {
		t.Fatalf("empty owner: want anon, got %q", fp)
	}
	a := ownerFingerprint("user_42")
	b := ownerFingerprint("user_42")
	if a != b {
		t.Fatalf("fingerprint not deterministic: %q vs %q", a, b)
	}
	if !strings.HasPrefix(a, "fp_") || len(a) != len("fp_")+12 {
		t.Fatalf("unexpected fingerprint shape: %q", a)
	}
	if ownerFingerprint("user_43") == a {
		t.Fatalf("fingerprint not unique for different owner")
	}
}

func TestFormatTime(t *testing.T) {
	if formatTime(time.Time{}) != "-" {
		t.Fatalf("zero time should render as '-'")
	}
	ts := time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC)
	if got := formatTime(ts); got != "2026-05-17T10:00:00Z" {
		t.Fatalf("formatTime = %q", got)
	}
}

func TestMergeNodes(t *testing.T) {
	counts := map[string]int{"a": 1, "z": 1, "b": 1}
	got := mergeNodes([]string{"b", "a"}, counts)
	if len(got) != 3 || got[0] != "b" || got[1] != "a" || got[2] != "z" {
		t.Fatalf("mergeNodes ordering wrong: %v", got)
	}
	got2 := mergeNodes([]string{"a", "a"}, map[string]int{})
	if len(got2) != 1 || got2[0] != "a" {
		t.Fatalf("dedupe failed: %v", got2)
	}
}

func TestPercentilesMs(t *testing.T) {
	if p50, p95 := percentilesMs(nil); p50 != 0 || p95 != 0 {
		t.Fatalf("empty case should be (0,0), got (%d,%d)", p50, p95)
	}
	obs := []observation{
		{Latency: 10 * time.Millisecond},
		{Latency: 20 * time.Millisecond},
		{Latency: 30 * time.Millisecond},
		{Latency: 40 * time.Millisecond},
		{Latency: 100 * time.Millisecond},
	}
	p50, p95 := percentilesMs(obs)
	if p50 != 30 {
		t.Fatalf("p50 = %d, want 30", p50)
	}
	if p95 != 100 {
		t.Fatalf("p95 = %d, want 100", p95)
	}
}

func TestNearestRank_BoundsClamp(t *testing.T) {
	xs := []int64{1, 2, 3}
	if got := nearestRank(xs, 0); got != 1 {
		t.Fatalf("rank for p=0 should clamp to first, got %d", got)
	}
	if got := nearestRank(xs, 1); got != 3 {
		t.Fatalf("rank for p=1 should clamp to last, got %d", got)
	}
	if got := nearestRank(nil, 0.5); got != 0 {
		t.Fatalf("empty input → 0, got %d", got)
	}
}

func TestDisclaimerLines_ASCIIFallbackIsAllASCII(t *testing.T) {
	for _, l := range disclaimerLines(false) {
		for _, r := range l {
			if r > 127 {
				t.Fatalf("non-ASCII rune %U in fallback line %q", r, l)
			}
		}
	}
}

func TestDisclaimerLines_CJKContainsRequiredPhrases(t *testing.T) {
	lines := disclaimerLines(true)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"本报告为 idcd 提供的一手观测数据",
		"Report Type: observation_only",
		"第三方使用本报告时",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("CJK disclaimer missing %q", want)
		}
	}
}

func TestLoadCJKFont_RejectsTTC(t *testing.T) {
	dir := t.TempDir()
	fpath := dir + "/fake.ttc"
	if err := os.WriteFile(fpath, append([]byte("ttcf"), make([]byte, 32)...), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	resetCJKFontForTest()
	t.Setenv("IDCD_PDF_CJK_FONT_PATH", fpath)
	defer resetCJKFontForTest()
	if got := loadCJKFont(); got.ok {
		t.Fatalf("ttc collection should be rejected")
	}
}

// TestRenderPDF_WithCJKFont covers the success branch of the font
// loader when a real TTF is available on the build host. Skipped when
// no system TTF is present so the test stays portable.
func TestRenderPDF_WithCJKFont(t *testing.T) {
	candidates := []string{
		"/usr/share/fonts/truetype/fonts-japanese-gothic.ttf",
		"/usr/share/fonts/opentype/ipafont-gothic/ipag.ttf",
	}
	var picked string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			picked = p
			break
		}
	}
	if picked == "" {
		t.Skip("no system CJK TTF available")
	}
	resetCJKFontForTest()
	t.Setenv("IDCD_PDF_CJK_FONT_PATH", picked)
	defer resetCJKFontForTest()

	if got := loadCJKFont(); !got.ok {
		t.Fatalf("expected font load to succeed for %s", picked)
	}
	out, err := renderPDF(fixedOrder("legal"), fixedObs(), nil)
	if err != nil {
		t.Fatalf("renderPDF: %v", err)
	}
	r, err := pdfreader.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("pdf.NewReader: %v", err)
	}
	if r.NumPage() < 3 {
		t.Fatalf("expected at least 3 pages, got %d", r.NumPage())
	}
}

func TestLoadCJKFont_MissingFileDegrades(t *testing.T) {
	resetCJKFontForTest()
	t.Setenv("IDCD_PDF_CJK_FONT_PATH", "/definitely/not/here.ttf")
	defer resetCJKFontForTest()
	if got := loadCJKFont(); got.ok {
		t.Fatalf("missing file should not load")
	}
}

// extractAllText walks every page and concatenates the per-glyph text
// runs into a single string. digitorus/pdf surfaces each glyph as a
// separate Text record, so callers should look for substring matches
// rather than expecting word boundaries.
func extractAllText(t *testing.T, r *pdfreader.Reader) string {
	t.Helper()
	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		content := page.Content()
		for _, tx := range content.Text {
			sb.WriteString(tx.S)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
