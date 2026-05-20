package sign

import (
	"crypto"
	"errors"
	"strings"
	"testing"
)

// 所有 sentinel error 必须非 nil 且以 "sign: " 前缀开头。这是约定俗成
// 的契约：上层把 err.Error() 入日志时凭前缀就能识别来源；正向锁死避免
// 后续重命名。
func TestSentinelErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"ErrUpstreamUnavailable", ErrUpstreamUnavailable},
		{"ErrAuthFailed", ErrAuthFailed},
		{"ErrInvalidInput", ErrInvalidInput},
		{"ErrKeyNotFound", ErrKeyNotFound},
		{"ErrSignatureInvalid", ErrSignatureInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.err == nil {
				t.Fatalf("%s is nil", c.name)
			}
			if !strings.HasPrefix(c.err.Error(), "sign: ") {
				t.Fatalf("%s does not have %q prefix: %q", c.name, "sign: ", c.err.Error())
			}
		})
	}
}

// HashAlgFor 必须对每个常量返回对应 stdlib hash，未知算法返回
// ErrInvalidInput。该函数是纯计算，必须 100% 覆盖。
func TestHashAlgFor(t *testing.T) {
	type want struct {
		hash crypto.Hash
		err  error
	}
	cases := []struct {
		alg  string
		want want
	}{
		{AlgorithmECDSASHA256, want{crypto.SHA256, nil}},
		{AlgorithmRSAPSSSHA256, want{crypto.SHA256, nil}},
		{AlgorithmRSAPKCS1SHA256, want{crypto.SHA256, nil}},
		{AlgorithmECDSASHA384, want{crypto.SHA384, nil}},
		{AlgorithmRSAPSSSHA384, want{crypto.SHA384, nil}},
		{AlgorithmECDSASHA512, want{crypto.SHA512, nil}},
		{AlgorithmRSAPSSSHA512, want{crypto.SHA512, nil}},
		{"", want{0, ErrInvalidInput}},
		{"UNKNOWN", want{0, ErrInvalidInput}},
		{"ecdsa_sha_256", want{0, ErrInvalidInput}}, // 大小写敏感
	}
	for _, c := range cases {
		t.Run(c.alg, func(t *testing.T) {
			got, err := HashAlgFor(c.alg)
			if !errors.Is(err, c.want.err) {
				t.Fatalf("err = %v, want %v", err, c.want.err)
			}
			if got != c.want.hash {
				t.Fatalf("hash = %v, want %v", got, c.want.hash)
			}
		})
	}
}

// expectedDigestLen 也是纯计算：返回算法对应 digest 字节数；未知算法 0。
func TestExpectedDigestLen(t *testing.T) {
	cases := []struct {
		alg  string
		want int
	}{
		{AlgorithmECDSASHA256, 32},
		{AlgorithmECDSASHA384, 48},
		{AlgorithmECDSASHA512, 64},
		{AlgorithmRSAPSSSHA256, 32},
		{AlgorithmRSAPSSSHA384, 48},
		{AlgorithmRSAPSSSHA512, 64},
		{AlgorithmRSAPKCS1SHA256, 32},
		{"", 0},
		{"UNKNOWN", 0},
	}
	for _, c := range cases {
		t.Run(c.alg, func(t *testing.T) {
			if got := expectedDigestLen(c.alg); got != c.want {
				t.Fatalf("expectedDigestLen(%q) = %d, want %d", c.alg, got, c.want)
			}
		})
	}
}
