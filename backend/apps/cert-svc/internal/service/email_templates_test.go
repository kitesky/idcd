package service

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleNotificationData() NotificationData {
	notAfter := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	return NotificationData{
		AccountID:    "42",
		CertID:       7,
		OrderID:      99,
		SANs:         []string{"example.com", "www.example.com"},
		CA:           "lets-encrypt",
		NotAfter:     &notAfter,
		DaysToExpire: 30,
		ErrorMsg:     "DNS lookup failed: nxdomain",
	}
}

func TestRenderIssued_ContainsAllFields(t *testing.T) {
	d := sampleNotificationData()
	subj, body := RenderIssued(d)

	require.NotEmpty(t, subj)
	require.NotEmpty(t, body)
	assert.Contains(t, subj, "签发成功")
	assert.Contains(t, subj, "example.com")

	assert.Contains(t, body, "example.com")
	assert.Contains(t, body, "www.example.com")
	assert.Contains(t, body, "lets-encrypt")
	assert.Contains(t, body, "2026-09-01")
	assert.Contains(t, body, "7")
	assert.Contains(t, body, "idcd.com/app/cert/7")
}

func TestRenderIssued_HandlesMissingOptionals(t *testing.T) {
	d := NotificationData{
		AccountID: "1",
		SANs:      []string{"a.test"},
	}
	subj, body := RenderIssued(d)

	assert.NotEmpty(t, subj)
	assert.Contains(t, body, "a.test")
	// no panic on missing CA / NotAfter / CertID
	assert.NotContains(t, body, "签发 CA：\n")
}

func TestRenderFailed_ContainsErrorAndOrderLink(t *testing.T) {
	d := sampleNotificationData()
	subj, body := RenderFailed(d)

	assert.Contains(t, subj, "签发失败")
	assert.Contains(t, body, "DNS lookup failed: nxdomain")
	assert.Contains(t, body, "example.com")
	assert.Contains(t, body, "idcd.com/app/cert/orders/99")
	assert.Contains(t, body, "lets-encrypt")
}

func TestRenderExpiring_MentionsBucketDays(t *testing.T) {
	for _, days := range []int{30, 14, 7, 1} {
		d := sampleNotificationData()
		d.DaysToExpire = days
		subj, body := RenderExpiring(d)

		assert.Contains(t, subj, "天后到期")
		// We allow either Chinese-formatted or plain numeric mention,
		// but the value MUST appear somewhere in subject/body.
		joined := subj + body
		assert.True(t, strings.Contains(joined, intToStr(days)),
			"expected days %d in %q", days, joined)
		assert.Contains(t, body, "example.com")
		assert.Contains(t, body, "2026-09-01")
	}
}

func TestRenderRenewalFailed_ContainsError(t *testing.T) {
	d := sampleNotificationData()
	d.ErrorMsg = "rate limited by CA"
	subj, body := RenderRenewalFailed(d)

	assert.Contains(t, subj, "续期失败")
	assert.Contains(t, body, "rate limited by CA")
	assert.Contains(t, body, "example.com")
	assert.Contains(t, body, "idcd.com/app/cert/7")
}

func TestRenderRenewalFailed_NoErrorMsg(t *testing.T) {
	d := NotificationData{
		AccountID: "1",
		CertID:    5,
		SANs:      []string{"a.test"},
	}
	_, body := RenderRenewalFailed(d)
	assert.Contains(t, body, "a.test")
	// no panic / no orphan "最近一次失败原因：" header when ErrorMsg empty
	assert.NotContains(t, body, "最近一次失败原因：\n")
}

func TestRenderNotification_DispatchesByEventType(t *testing.T) {
	d := sampleNotificationData()

	d.EventType = EventCertIssued
	s, b := RenderNotification(d)
	assert.Contains(t, s, "签发成功")
	assert.NotEmpty(t, b)

	d.EventType = EventCertFailed
	s, b = RenderNotification(d)
	assert.Contains(t, s, "签发失败")
	assert.NotEmpty(t, b)

	d.EventType = EventCertExpiring
	s, b = RenderNotification(d)
	assert.Contains(t, s, "天后到期")
	assert.NotEmpty(t, b)

	d.EventType = EventCertRenewalFailed
	s, b = RenderNotification(d)
	assert.Contains(t, s, "续期失败")
	assert.NotEmpty(t, b)
}

func TestRenderNotification_UnknownEventTypeReturnsEmpty(t *testing.T) {
	s, b := RenderNotification(NotificationData{EventType: "cert.revoked"})
	// revoked is intentionally not rendered as an email template — it
	// flows through the watcher with empty subject/body and the
	// downstream notifier decides what to do.
	assert.Empty(t, s)
	assert.Empty(t, b)
}

func TestSanList_EmptyReturnsPlaceholder(t *testing.T) {
	assert.Equal(t, "(无)", sanList(nil))
	assert.Equal(t, "(无)", sanList([]string{}))
	assert.Equal(t, "a, b", sanList([]string{"a", "b"}))
}

func TestFormatTime_NilReturnsPlaceholder(t *testing.T) {
	assert.Equal(t, "(未知)", formatTime(nil))
	tt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	assert.Contains(t, formatTime(&tt), "2026-01-02")
}

func TestOrderAndCertLinks_FallbackOnZero(t *testing.T) {
	assert.Equal(t, "https://idcd.com/app/cert/orders", orderLink(0))
	assert.Equal(t, "https://idcd.com/app/cert", certLink(0))
	assert.Equal(t, "https://idcd.com/app/cert/orders/42", orderLink(42))
	assert.Equal(t, "https://idcd.com/app/cert/9", certLink(9))
}

// intToStr is a tiny helper to avoid importing strconv just for tests.
func intToStr(n int) string {
	// Hand-rolled to keep imports minimal.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
