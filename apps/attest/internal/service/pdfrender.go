package service

// renderPDF stubs step 4. The real implementation will use a templated
// HTML→PDF renderer; here we hand-build the minimal PDF 1.4 layout that
// satisfies the magic-byte check in lib/attest/pdfsign and parses
// cleanly via github.com/digitorus/pdf.
//
// The byte layout is copied from the test fixture in
// lib/attest/pdfsign/pdfsign_test.go (minimalPDF) — DO NOT edit without
// recomputing xref offsets.
func renderPDF(_ *Order, _ []observation, _ []string) ([]byte, error) {
	const minimalPDF = "%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> >>\nendobj\n" +
		"xref\n" +
		"0 4\n" +
		"0000000000 65535 f \n" +
		"0000000009 00000 n \n" +
		"0000000058 00000 n \n" +
		"0000000115 00000 n \n" +
		"trailer\n<< /Size 4 /Root 1 0 R >>\n" +
		"startxref\n203\n" +
		"%%EOF\n"
	return []byte(minimalPDF), nil
}
