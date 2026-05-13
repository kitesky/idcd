package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kite365/idcd/apps/notifier/internal/config"
	"github.com/kite365/idcd/apps/notifier/internal/email"
	"github.com/kite365/idcd/apps/notifier/internal/template"
	"github.com/kite365/idcd/apps/notifier/internal/worker"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

func main() {
	// Load configuration
	cfg := config.MustLoad("config/dev.env.yaml") // or use IDCD_CONFIG env var

	// Initialize logger
	log := logger.New(cfg.Server.Env)

	// Initialize OpenTelemetry
	telCfg := telemetry.Config{
		ServiceName:    "idcd-notifier",
		ServiceVersion: "v1.0.0",
		OTLPEndpoint:   cfg.Observability.Telemetry.OTLPEndpoint,
		SamplingRate:   cfg.Observability.Telemetry.SamplingRate,
		Enabled:        cfg.Observability.Telemetry.Enabled,
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		log.Error("failed to init telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	// Initialize templates
	templates, err := template.New()
	if err != nil {
		log.Error("初始化邮件模板失败", "error", err)
		os.Exit(1)
	}

	// Initialize email sender (prefer SMTP, fallback to SES if configured)
	var emailSender email.Sender
	if cfg.Notifier.SMTP.Host != "" {
		emailSender = email.NewSMTPSender(email.SMTPConfig{
			Host:     cfg.Notifier.SMTP.Host,
			Port:     cfg.Notifier.SMTP.Port,
			Username: cfg.Notifier.SMTP.Username,
			Password: cfg.Notifier.SMTP.Password,
			From:     cfg.Notifier.SMTP.From,
			FromName: cfg.Notifier.SMTP.FromName,
		})
		log.Info("使用 SMTP 发送邮件", "host", cfg.Notifier.SMTP.Host, "port", cfg.Notifier.SMTP.Port)
	} else if cfg.Notifier.SES.Region != "" {
		emailSender = email.NewSESSender(email.SESConfig{
			Region:    cfg.Notifier.SES.Region,
			AccessKey: cfg.Notifier.SES.AccessKey,
			SecretKey: cfg.Notifier.SES.SecretKey,
			From:      cfg.Notifier.SES.From,
			FromName:  cfg.Notifier.SES.FromName,
		})
		log.Info("使用 AWS SES 发送邮件", "region", cfg.Notifier.SES.Region)
	} else {
		log.Error("未配置邮件发送方式，请设置 SMTP 或 SES 配置")
		os.Exit(1)
	}

	// Initialize handlers
	handlers := worker.NewHandlers(emailSender, templates, log)

	// Initialize worker
	w, err := worker.NewWorker(&cfg.Notifier, handlers, log)
	if err != nil {
		log.Error("初始化邮件 Worker 失败", "error", err)
		os.Exit(1)
	}

	// Set up graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start worker
	go func() {
		if err := w.Start(ctx); err != nil {
			log.Error("启动邮件 Worker 失败", "error", err)
			cancel()
		}
	}()

	log.Info("邮件通知服务已启动，等待任务...")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Info("收到停止信号，正在优雅关闭...")

	// Stop worker gracefully
	if err := w.Stop(context.Background()); err != nil {
		log.Error("停止邮件 Worker 失败", "error", err)
		os.Exit(1)
	}

	log.Info("邮件通知服务已停止")
}

