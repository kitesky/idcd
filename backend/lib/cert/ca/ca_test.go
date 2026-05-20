package ca

import (
	"errors"
	"testing"
)

func TestTierConstants(t *testing.T) {
	cases := map[Tier]string{
		TierFreeDV: "free-dv",
		TierPaidDV: "paid-dv",
		TierPaidOV: "paid-ov",
		TierPaidEV: "paid-ev",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("tier wire value = %q, want %q", string(got), want)
		}
	}
}

func TestChallengeTypeConstants(t *testing.T) {
	cases := map[ChallengeType]string{
		ChallengeDNS01:  "dns-01",
		ChallengeHTTP01: "http-01",
		ChallengeEmail:  "email",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("challenge wire value = %q, want %q", string(got), want)
		}
	}
}

func TestRevokeReasonConstants(t *testing.T) {
	cases := map[RevokeReason]int{
		RevokeUnspecified:          0,
		RevokeKeyCompromise:        1,
		RevokeCessationOfOperation: 5,
		RevokeCertificateHold:      6,
	}
	for got, want := range cases {
		if int(got) != want {
			t.Errorf("revoke reason code = %d, want %d (RFC 5280 §5.3.1)", int(got), want)
		}
	}
}

func TestSentinelsAreUnique(t *testing.T) {
	all := []error{
		ErrCAQuotaExceeded,
		ErrAuthzInvalid,
		ErrCAATooStrict,
		ErrAccountInvalid,
		ErrNetwork,
		ErrInvalidInput,
	}
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("sentinel %d (%v) collides with sentinel %d (%v)", i, a, j, b)
			}
		}
	}
}

func TestSentinelsAreNonNil(t *testing.T) {
	all := []error{
		ErrCAQuotaExceeded,
		ErrAuthzInvalid,
		ErrCAATooStrict,
		ErrAccountInvalid,
		ErrNetwork,
		ErrInvalidInput,
	}
	for i, e := range all {
		if e == nil {
			t.Fatalf("sentinel %d is nil", i)
		}
		if e.Error() == "" {
			t.Errorf("sentinel %d has empty message", i)
		}
	}
}
