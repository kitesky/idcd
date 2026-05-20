package email

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	smithy "github.com/aws/smithy-go"

	"github.com/kite365/idcd/lib/shared/apperr"
)

// SESConfig holds AWS SES configuration.
//
// AccessKey/SecretKey are optional: if both are empty, the SDK falls back to
// the default credential chain (env vars, IAM role, shared profile, IMDS,
// …) via config.LoadDefaultConfig. Set them explicitly only when running
// outside AWS without an attached IAM role.
type SESConfig struct {
	Region    string `yaml:"region"`     // AWS region (e.g., "us-east-1")
	AccessKey string `yaml:"access_key"` // AWS access key ID (optional)
	SecretKey string `yaml:"secret_key"` // AWS secret access key (optional)
	From      string `yaml:"from"`       // sender email address
	FromName  string `yaml:"from_name"`  // sender display name
}

// sesAPI is the subset of the sesv2.Client surface the sender uses.
// Kept as an interface so tests can inject a fake client without
// standing up a real HTTP server.
type sesAPI interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// SESSender implements the Sender interface using AWS SES v2.
type SESSender struct {
	config SESConfig
	client sesAPI
	// initErr captures any config / credential resolution failure that
	// happened in the constructor. When non-nil, every Send() call returns
	// it unchanged — classified as Internal so asynq's retry kicks in
	// (the operator can fix the config without losing queued mail).
	initErr error
}

// Option customises SESSender construction. Currently only used by tests
// (see withHTTPClient) but retained as a public extension point.
type Option func(*sesSenderOpts)

type sesSenderOpts struct {
	httpClient sesv2.HTTPClient
	clientOpts []func(*sesv2.Options)
	client     sesAPI // pre-built client override (used by tests)
}

// WithHTTPClient injects a custom HTTP client used for SES transport.
// This is the primary seam tests use to intercept SES wire calls via
// httptest.Server without hitting real AWS endpoints.
func WithHTTPClient(c sesv2.HTTPClient) Option {
	return func(o *sesSenderOpts) { o.httpClient = c }
}

// WithSESClient injects an entire fake sesv2-shaped client. Tests can
// avoid wire serialisation altogether by returning canned responses.
// Not exposed via documentation comments outside the package internals.
func WithSESClient(c sesAPI) Option {
	return func(o *sesSenderOpts) { o.client = c }
}

// NewSESSender creates a new AWS SES v2 email sender.
//
// Construction never fails fatally: if AWS credential resolution errors
// (e.g. missing static creds AND no default chain available), the error
// is stored on the sender and surfaced from Send() — that way main.go's
// wiring stays a simple builder and asynq's retry covers operator misconfig.
func NewSESSender(cfg SESConfig, opts ...Option) *SESSender {
	s := &SESSender{config: cfg}

	o := &sesSenderOpts{}
	for _, opt := range opts {
		opt(o)
	}

	// Test-only short-circuit: caller wired a complete fake client.
	if o.client != nil {
		s.client = o.client
		return s
	}

	awsCfg, err := loadAWSConfig(cfg, o)
	if err != nil {
		s.initErr = err
		return s
	}
	s.client = sesv2.NewFromConfig(awsCfg, o.clientOpts...)
	return s
}

// loadAWSConfig resolves the AWS SDK base config.
//
//   - Explicit AccessKey+SecretKey → static credentials provider.
//   - Otherwise → default chain (env → shared profile → IMDS → …).
//
// Region is always taken from SESConfig; an empty region is treated as a
// permanent configuration error so we fail fast at construction rather
// than at the first send.
func loadAWSConfig(cfg SESConfig, o *sesSenderOpts) (aws.Config, error) {
	if cfg.Region == "" {
		return aws.Config{}, fmt.Errorf("SES region 未配置")
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		loadOpts = append(loadOpts,
			awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
			),
		)
	}
	if o.httpClient != nil {
		loadOpts = append(loadOpts, awsconfig.WithHTTPClient(o.httpClient))
	}

	return awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
}

// Send sends an email using AWS SES v2.
//
// Error classification follows the same retry contract as the SMTP sender
// so the asynq email queue treats failures consistently:
//   - apperr.Validation  → permanent, do not retry (bad input / config / addr).
//   - apperr.Unauthorized → permanent, do not retry (bad credentials).
//   - apperr.Unavailable  → transient, retry (throttle / 5xx / network).
//   - apperr.Internal     → retry (defensive default for unknown errors).
func (s *SESSender) Send(ctx context.Context, msg Message) error {
	if s.initErr != nil {
		// Surface as Internal so asynq retries — the cause is a
		// recoverable misconfig (operator fixes config + restart).
		return apperr.Internal("SES 客户端初始化失败", s.initErr)
	}
	if err := s.validateMessage(msg); err != nil {
		return err
	}

	from := s.buildFromAddress()
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination: &sestypes.Destination{
			ToAddresses: []string{msg.To},
		},
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{
					Data:    aws.String(msg.Subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &sestypes.Body{
					Html: &sestypes.Content{
						Data:    aws.String(msg.HTML),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	if _, err := s.client.SendEmail(ctx, input); err != nil {
		return classifySESError(err)
	}
	return nil
}

// validateMessage mirrors SMTPSender.validateMessage so both senders enforce
// the same minimum payload before going on the wire.
func (s *SESSender) validateMessage(msg Message) error {
	if msg.To == "" {
		return apperr.Validation("收件人地址不能为空", "")
	}
	if msg.Subject == "" {
		return apperr.Validation("邮件主题不能为空", "")
	}
	if msg.HTML == "" {
		return apperr.Validation("邮件内容不能为空", "")
	}
	return nil
}

// buildFromAddress constructs "Name <email>" form when a display name is set.
// SES v2 accepts RFC 5322 mailbox syntax in FromEmailAddress.
func (s *SESSender) buildFromAddress() string {
	if s.config.FromName != "" {
		return fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)
	}
	return s.config.From
}

// classifySESError maps an AWS SDK error to the right apperr.Code so the
// notifier's retry policy makes sensible decisions.
//
// Rules:
//   - Throttling (TooManyRequestsException) → Unavailable (retry).
//   - SendingPausedException                → Unavailable (account-level pause is
//     usually transient; retry lets things heal once SES re-enables sending).
//   - InternalServiceErrorException         → Unavailable (server-side, retry).
//   - AccountSuspendedException             → Unauthorized (account-killed, no retry).
//   - BadRequestException / MailFromDomain  → Validation (permanent input/config).
//   - smithy.APIError with FaultServer      → Unavailable (retry 5xx).
//   - smithy.APIError with FaultClient      → Validation (permanent 4xx).
//   - Anything else (network / context)     → Unavailable (retry — safer default).
func classifySESError(err error) error {
	if err == nil {
		return nil
	}

	// Named SES v2 exceptions first — most precise classification.
	var (
		tooMany   *sestypes.TooManyRequestsException
		paused    *sestypes.SendingPausedException
		intSvc    *sestypes.InternalServiceErrorException
		suspended *sestypes.AccountSuspendedException
		badReq    *sestypes.BadRequestException
		mfdnv     *sestypes.MailFromDomainNotVerifiedException
		limit     *sestypes.LimitExceededException
	)
	switch {
	case errors.As(err, &tooMany):
		return apperr.Unavailable("SES 限流，请稍后重试", err)
	case errors.As(err, &paused):
		return apperr.Unavailable("SES 账户暂停发送，稍后重试", err)
	case errors.As(err, &intSvc):
		return apperr.Unavailable("SES 服务端错误", err)
	case errors.As(err, &suspended):
		return apperr.Unauthorized("SES 账户已被暂停")
	case errors.As(err, &badReq):
		return apperr.Validation("SES 请求无效（地址或内容不合法）", safeAPIDetail(err))
	case errors.As(err, &mfdnv):
		return apperr.Validation("SES MAIL FROM 域名未验证", safeAPIDetail(err))
	case errors.As(err, &limit):
		// SES LimitExceeded is a hard configuration limit, not a rate
		// limit — needs operator action, not retry.
		return apperr.Validation("SES 配额超限（需运维介入）", safeAPIDetail(err))
	}

	// Fall back to smithy APIError fault classification.
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorFault() {
		case smithy.FaultServer:
			return apperr.Unavailable("SES 服务端错误："+apiErr.ErrorCode(), err)
		case smithy.FaultClient:
			return apperr.Validation("SES 客户端错误："+apiErr.ErrorCode(), apiErr.ErrorMessage())
		}
	}

	// Network / context / unknown wire failures — retry.
	return apperr.Unavailable("SES 调用失败", err)
}

// safeAPIDetail extracts a non-empty, single-line detail string from a smithy
// API error for inclusion in the apperr.Detail field (which downstream
// templating treats as user-facing).
func safeAPIDetail(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		msg := apiErr.ErrorMessage()
		// Collapse newlines so logs / dashboards stay grep-friendly.
		msg = strings.ReplaceAll(msg, "\n", " ")
		msg = strings.ReplaceAll(msg, "\r", " ")
		return msg
	}
	return ""
}

// Compile-time interface check: an *http.Client satisfies sesv2.HTTPClient.
// Keeps WithHTTPClient ergonomic for tests using httptest.Server.
var _ sesv2.HTTPClient = (*http.Client)(nil)
