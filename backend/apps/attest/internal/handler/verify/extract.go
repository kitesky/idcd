// Package verify implements the public /verify endpoint for idcd's
// S2 Attestation pipeline. It accepts a signed PDF (multipart upload)
// or a known report_id, extracts the embedded CMS SignedData + RFC3161
// TimeStampToken, and verifies them against the KMS public key.
//
// D6 mandate: the Self-Verify Worker MUST exercise this exact path
// (HTTPS + public endpoint). The handler therefore makes no internal
// shortcut and assumes the caller is untrusted.
package verify

import (
	"bytes"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/digitorus/pkcs7"
	"github.com/digitorus/timestamp"
)

// Sentinel errors returned by the extract helpers. The HTTP handler
// translates these into VerifyResult.Reason strings.
var (
	// ErrNotPDF is returned when the input lacks the %PDF- magic.
	ErrNotPDF = errors.New("not a pdf")
	// ErrNoSignature indicates no /Sig dictionary (or no /ByteRange
	// + /Contents pair) was found in the PDF.
	ErrNoSignature = errors.New("no signature found in pdf")
	// ErrByteRangeMalformed indicates the /ByteRange array could not
	// be parsed as four non-negative integers.
	ErrByteRangeMalformed = errors.New("malformed /ByteRange")
	// ErrContentsMalformed indicates the /Contents hex string is
	// missing or not valid hex.
	ErrContentsMalformed = errors.New("malformed /Contents")
)

// rfc3161TSATokenOID is the id-aa-timeStampToken unsigned attribute
// (RFC 3161 §4.2 / RFC 5126 §6.1.1).
var rfc3161TSATokenOID = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}

// pdfMagic returns true when buf starts with the %PDF- header.
func pdfMagic(buf []byte) bool {
	return len(buf) >= 5 && bytes.Equal(buf[:5], []byte("%PDF-"))
}

// extracted bundles everything the verifier needs from a PDF.
type extracted struct {
	SignedBytes []byte // the byte ranges that were hashed by the signer
	CMS         []byte // raw CMS SignedData bytes (decoded from /Contents)
	ByteRange   [4]int64
}

// extract parses pdfBytes and pulls out the signed byte range plus the
// CMS blob. It does not perform any cryptographic operations.
func extract(pdfBytes []byte) (*extracted, error) {
	if !pdfMagic(pdfBytes) {
		return nil, ErrNotPDF
	}
	br, err := parseByteRange(pdfBytes)
	if err != nil {
		return nil, err
	}
	cms, err := extractContents(pdfBytes)
	if err != nil {
		return nil, err
	}
	signed, err := assembleSignedBytes(pdfBytes, br)
	if err != nil {
		return nil, err
	}
	return &extracted{
		SignedBytes: signed,
		CMS:         cms,
		ByteRange:   br,
	}, nil
}

// byteRangeRE matches `/ByteRange [a b c d]` with arbitrary
// whitespace (spaces, tabs, newlines) inside the brackets. PDF
// writers occasionally pad with spaces so the array width can stay
// constant after the placeholder /Contents is filled in.
var byteRangeRE = regexp.MustCompile(`/ByteRange\s*\[\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s*\]`)

// parseByteRange finds the /ByteRange array in the PDF and returns
// the four offsets. PAdES signatures always have exactly two ranges
// (= 4 integers): [offset1 length1 offset2 length2] where the
// excluded gap is the /Contents placeholder.
func parseByteRange(pdfBytes []byte) ([4]int64, error) {
	var out [4]int64
	m := byteRangeRE.FindSubmatch(pdfBytes)
	if m == nil {
		return out, fmt.Errorf("%w: /ByteRange not found", ErrByteRangeMalformed)
	}
	for i := 0; i < 4; i++ {
		n, err := strconv.ParseInt(string(m[i+1]), 10, 64)
		if err != nil || n < 0 {
			return out, fmt.Errorf("%w: index %d not a non-negative integer", ErrByteRangeMalformed, i)
		}
		out[i] = n
	}
	return out, nil
}

// extractContents finds the /Contents <hex...> entry that immediately
// follows a /ByteRange and decodes the hex blob into raw CMS bytes.
//
// The PDF spec permits two forms — `<hex>` and `(literal)` — but
// PAdES / Adobe always emit the hex form for /Contents because the
// CMS blob would otherwise be unparseable through PDF string escaping.
func extractContents(pdfBytes []byte) ([]byte, error) {
	// We deliberately don't anchor against /ByteRange because some
	// writers emit /Contents before /ByteRange. Instead we find every
	// /Contents <...> entry and pick the longest one — the CMS blob
	// is always much larger than any other /Contents string in a
	// typical PDF.
	contentsRE := regexp.MustCompile(`/Contents\s*<([0-9A-Fa-f\s]*)>`)
	matches := contentsRE.FindAllSubmatch(pdfBytes, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: /Contents hex string not found", ErrContentsMalformed)
	}
	var best []byte
	for _, m := range matches {
		// Strip whitespace inside the hex blob — some PDFs wrap it.
		cleaned := make([]byte, 0, len(m[1]))
		for _, b := range m[1] {
			if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
				cleaned = append(cleaned, b)
			}
		}
		// Pad with a trailing zero nibble if odd-length (defensive;
		// signers usually pad on their own).
		if len(cleaned)%2 == 1 {
			cleaned = append(cleaned, '0')
		}
		decoded := make([]byte, hex.DecodedLen(len(cleaned)))
		n, err := hex.Decode(decoded, cleaned)
		if err != nil {
			continue
		}
		decoded = decoded[:n]
		// Trim trailing zero padding (PAdES /Contents is fixed-width
		// with the remainder filled with 0x00 after the CMS blob).
		decoded = bytes.TrimRight(decoded, "\x00")
		if len(decoded) > len(best) {
			best = decoded
		}
	}
	if len(best) == 0 {
		return nil, fmt.Errorf("%w: /Contents hex decoded to empty", ErrContentsMalformed)
	}
	return best, nil
}

// assembleSignedBytes concatenates the two byte ranges identified by
// the /ByteRange array. This is the exact octet stream the signer
// hashed before submitting to KMS.
func assembleSignedBytes(pdfBytes []byte, br [4]int64) ([]byte, error) {
	total := int64(len(pdfBytes))
	if br[0] < 0 || br[2] < 0 || br[1] < 0 || br[3] < 0 {
		return nil, fmt.Errorf("%w: negative offset/length", ErrByteRangeMalformed)
	}
	if br[0]+br[1] > total || br[2]+br[3] > total {
		return nil, fmt.Errorf("%w: range extends past pdf end (size=%d)", ErrByteRangeMalformed, total)
	}
	out := make([]byte, 0, br[1]+br[3])
	out = append(out, pdfBytes[br[0]:br[0]+br[1]]...)
	out = append(out, pdfBytes[br[2]:br[2]+br[3]]...)
	return out, nil
}

// extractTSAToken pulls the id-aa-timeStampToken unsigned attribute
// (if any) out of the CMS SignedData and parses it. Returns (nil,
// nil) when the signature is plain PAdES-B (no embedded TSA).
func extractTSAToken(cms []byte) (*timestamp.Timestamp, error) {
	p7, err := pkcs7.Parse(cms)
	if err != nil {
		return nil, fmt.Errorf("parse cms: %w", err)
	}
	for _, s := range p7.Signers {
		for _, attr := range s.UnauthenticatedAttributes {
			if !attr.Type.Equal(rfc3161TSATokenOID) {
				continue
			}
			ts, err := timestamp.Parse(attr.Value.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse tsa token: %w", err)
			}
			return ts, nil
		}
	}
	return nil, nil
}
