package verify

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPDFMagic(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want bool
	}{
		{"valid 1.4", []byte("%PDF-1.4\nfoo"), true},
		{"valid 2.0", []byte("%PDF-2.0\n"), true},
		{"too short", []byte("%PD"), false},
		{"wrong magic", []byte("PK\x03\x04"), false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pdfMagic(c.in); got != c.want {
				t.Fatalf("pdfMagic(%q)=%v want %v", c.in, got, c.want)
			}
		})
	}
}

func TestParseByteRange(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want [4]int64
		err  bool
	}{
		{
			name: "compact",
			in:   `xx /ByteRange[0 100 200 300] xx`,
			want: [4]int64{0, 100, 200, 300},
		},
		{
			name: "spaces",
			in:   `xx /ByteRange [ 0 100 200 300 ] xx`,
			want: [4]int64{0, 100, 200, 300},
		},
		{
			name: "tabs and newlines",
			in:   "xx /ByteRange\t[\n0\t100\n200 300\n] xx",
			want: [4]int64{0, 100, 200, 300},
		},
		{
			name: "large numbers",
			in:   `/ByteRange [0 12345 67890 1000000]`,
			want: [4]int64{0, 12345, 67890, 1000000},
		},
		{
			name: "missing",
			in:   `no byterange here`,
			err:  true,
		},
		{
			name: "only three",
			in:   `/ByteRange [0 100 200]`,
			err:  true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseByteRange([]byte(c.in))
			if c.err {
				if !errors.Is(err, ErrByteRangeMalformed) {
					t.Fatalf("expected ErrByteRangeMalformed, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("parseByteRange=%v want %v", got, c.want)
			}
		})
	}
}

func TestExtractContents(t *testing.T) {
	// Build a CMS-like payload (just arbitrary bytes) and embed it as
	// hex in a /Contents string. Pad with trailing zeros to simulate
	// PAdES fixed-width /Contents.
	payload := []byte{0x30, 0x82, 0x01, 0x23, 0xAA, 0xBB, 0xCC}
	hexBuf := &bytes.Buffer{}
	for _, b := range payload {
		// upper-case hex matches what most signers emit
		hexBuf.WriteString(toHex(b))
	}
	// Trailing zero padding (decoded must be trimmed)
	hexBuf.WriteString("0000000000")

	pdf := "%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Sig /Contents <" + hexBuf.String() + "> /ByteRange [0 10 100 200] >>\nendobj\n%%EOF\n"

	got, err := extractContents([]byte(pdf))
	if err != nil {
		t.Fatalf("extractContents: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("extractContents=% x want % x", got, payload)
	}
}

func TestExtractContents_WithWhitespace(t *testing.T) {
	// Some signers split the hex across lines; extractContents must
	// strip whitespace before decoding.
	hexBlob := "DEAD\nBEEF\n"
	pdf := "%PDF-1.4\n/Contents <" + hexBlob + ">\n%%EOF\n"
	got, err := extractContents([]byte(pdf))
	if err != nil {
		t.Fatalf("extractContents: %v", err)
	}
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(got, want) {
		t.Fatalf("extractContents=% x want % x", got, want)
	}
}

func TestExtractContents_Missing(t *testing.T) {
	_, err := extractContents([]byte("%PDF-1.4\nno contents here\n%%EOF"))
	if !errors.Is(err, ErrContentsMalformed) {
		t.Fatalf("expected ErrContentsMalformed, got %v", err)
	}
}

func TestExtractContents_AllZeros(t *testing.T) {
	pdf := "%PDF-1.4\n/Contents <0000000000>\n%%EOF"
	_, err := extractContents([]byte(pdf))
	if !errors.Is(err, ErrContentsMalformed) {
		t.Fatalf("expected ErrContentsMalformed for all-zero contents, got %v", err)
	}
}

func TestExtractContents_PicksLongest(t *testing.T) {
	// PDFs typically contain other /Contents entries (e.g. page
	// streams won't, but form fields might). Ensure we pick the
	// largest blob — the signature is always the biggest.
	tiny := "0102"
	big := strings.Repeat("AB", 200)
	pdf := "%PDF-1.4\n/Contents <" + tiny + ">\n...\n/Contents <" + big + ">\n%%EOF"
	got, err := extractContents([]byte(pdf))
	if err != nil {
		t.Fatalf("extractContents: %v", err)
	}
	if len(got) != 200 {
		t.Fatalf("expected 200 bytes, got %d", len(got))
	}
}

func TestAssembleSignedBytes(t *testing.T) {
	pdf := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	// Skip "CDEFG" (5 bytes starting at offset 2): include [0,2) and [7,end).
	got, err := assembleSignedBytes(pdf, [4]int64{0, 2, 7, int64(len(pdf)) - 7})
	if err != nil {
		t.Fatalf("assembleSignedBytes: %v", err)
	}
	want := []byte("ABHIJKLMNOPQRSTUVWXYZ")
	if !bytes.Equal(got, want) {
		t.Fatalf("assembleSignedBytes=%q want %q", got, want)
	}
}

func TestAssembleSignedBytes_OutOfBounds(t *testing.T) {
	pdf := []byte("ABCDEFGHIJ")
	_, err := assembleSignedBytes(pdf, [4]int64{0, 5, 100, 50})
	if !errors.Is(err, ErrByteRangeMalformed) {
		t.Fatalf("expected ErrByteRangeMalformed for OOB range, got %v", err)
	}
}

func TestExtract_NonPDF(t *testing.T) {
	_, err := extract([]byte("not a pdf at all"))
	if !errors.Is(err, ErrNotPDF) {
		t.Fatalf("expected ErrNotPDF, got %v", err)
	}
}

func TestExtract_NoByteRange(t *testing.T) {
	_, err := extract([]byte("%PDF-1.4\nno sig here\n%%EOF"))
	if !errors.Is(err, ErrByteRangeMalformed) {
		t.Fatalf("expected ErrByteRangeMalformed, got %v", err)
	}
}

func TestExtract_NoContents(t *testing.T) {
	_, err := extract([]byte("%PDF-1.4\n/ByteRange [0 10 100 200]\nbut no contents\n%%EOF"))
	if !errors.Is(err, ErrContentsMalformed) {
		t.Fatalf("expected ErrContentsMalformed, got %v", err)
	}
}

func TestExtractTSAToken_NotCMS(t *testing.T) {
	// Garbage bytes — pkcs7.Parse should reject them.
	_, err := extractTSAToken([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Fatal("expected parse error for non-CMS bytes")
	}
}

// toHex returns the uppercase hex of one byte without using
// encoding/hex (keeps the test self-contained for easy diffing).
func toHex(b byte) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{digits[b>>4], digits[b&0x0F]})
}
