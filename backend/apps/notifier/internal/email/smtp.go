package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/kite365/idcd/lib/shared/apperr"
)

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	Host     string `yaml:"host"`     // SMTP server host
	Port     int    `yaml:"port"`     // SMTP server port (587 for STARTTLS, 465 for TLS)
	Username string `yaml:"username"` // SMTP authentication username
	Password string `yaml:"password"` // SMTP authentication password
	From     string `yaml:"from"`     // sender email address
	FromName string `yaml:"from_name"` // sender display name
}

// SMTPSender implements the Sender interface using SMTP.
type SMTPSender struct {
	config SMTPConfig
}

// NewSMTPSender creates a new SMTP email sender.
func NewSMTPSender(config SMTPConfig) *SMTPSender {
	return &SMTPSender{config: config}
}

// Send sends an email using SMTP.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if err := s.validateMessage(msg); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Build the message
	fromAddr := s.buildFromAddress()
	msgBytes := s.buildMessage(fromAddr, msg)

	// Send based on port configuration
	switch s.config.Port {
	case 465:
		// Direct TLS connection
		return s.sendTLS(ctx, addr, fromAddr, msg.To, msgBytes)
	case 587:
		// STARTTLS
		return s.sendSTARTTLS(ctx, addr, fromAddr, msg.To, msgBytes)
	default:
		// Default to STARTTLS for other ports
		return s.sendSTARTTLS(ctx, addr, fromAddr, msg.To, msgBytes)
	}
}

// sendSTARTTLS sends email using STARTTLS (port 587).
func (s *SMTPSender) sendSTARTTLS(ctx context.Context, addr, from, to string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return apperr.Unavailable("SMTP连接失败", err)
	}
	defer client.Quit()

	// STARTTLS
	if err := client.StartTLS(&tls.Config{
		ServerName: s.config.Host,
	}); err != nil {
		return apperr.Unavailable("STARTTLS升级失败", err)
	}

	// Auth
	if s.config.Username != "" {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		if err := client.Auth(auth); err != nil {
			return apperr.Unauthorized("SMTP认证失败")
		}
	}

	// Send
	if err := client.Mail(from); err != nil {
		return apperr.Internal("设置发送方失败", err)
	}
	if err := client.Rcpt(to); err != nil {
		return apperr.Internal("设置收件方失败", err)
	}

	w, err := client.Data()
	if err != nil {
		return apperr.Internal("开始数据传输失败", err)
	}
	defer w.Close()

	if _, err := w.Write(msg); err != nil {
		return apperr.Internal("发送邮件内容失败", err)
	}

	return nil
}

// sendTLS sends email using direct TLS connection (port 465).
func (s *SMTPSender) sendTLS(ctx context.Context, addr, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: s.config.Host,
	})
	if err != nil {
		return apperr.Unavailable("TLS连接失败", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return apperr.Unavailable("SMTP客户端创建失败", err)
	}
	defer client.Quit()

	// Auth
	if s.config.Username != "" {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		if err := client.Auth(auth); err != nil {
			return apperr.Unauthorized("SMTP认证失败")
		}
	}

	// Send
	if err := client.Mail(from); err != nil {
		return apperr.Internal("设置发送方失败", err)
	}
	if err := client.Rcpt(to); err != nil {
		return apperr.Internal("设置收件方失败", err)
	}

	w, err := client.Data()
	if err != nil {
		return apperr.Internal("开始数据传输失败", err)
	}
	defer w.Close()

	if _, err := w.Write(msg); err != nil {
		return apperr.Internal("发送邮件内容失败", err)
	}

	return nil
}

// validateMessage validates the email message.
func (s *SMTPSender) validateMessage(msg Message) error {
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

// buildFromAddress constructs the "From" address with optional display name.
func (s *SMTPSender) buildFromAddress() string {
	if s.config.FromName != "" {
		return fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)
	}
	return s.config.From
}

// buildMessage constructs the full MIME message.
func (s *SMTPSender) buildMessage(from string, msg Message) []byte {
	var b strings.Builder

	// Headers
	b.WriteString(fmt.Sprintf("From: %s\r\n", from))
	b.WriteString(fmt.Sprintf("To: %s\r\n", msg.To))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")

	// Body
	b.WriteString(msg.HTML)

	return []byte(b.String())
}