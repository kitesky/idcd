// worker is the cert-svc ACME orchestrator.
//
// S1 W3: drives the Redis-Stream consumer + the order state machine, and
// runs the manual-TXT Redis pub/sub subscriber so the HTTP server can
// notify this process when a user installs a DNS challenge record. The
// ACME account key is loaded from cert.acme_accounts (envelope-encrypted
// via vault); first start generates and persists, subsequent restarts
// decrypt the existing row.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/cert-svc/internal/config"
	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/ca/buypass"
	"github.com/kite365/idcd/lib/cert/ca/letsencrypt"
	"github.com/kite365/idcd/lib/cert/ca/zerossl"
	"github.com/kite365/idcd/lib/cert/dns"
	"github.com/kite365/idcd/lib/cert/dns/aliyun"
	"github.com/kite365/idcd/lib/cert/dns/cloudflare"
	"github.com/kite365/idcd/lib/cert/dns/dnspod"
	"github.com/kite365/idcd/lib/cert/dns/gcloud"
	"github.com/kite365/idcd/lib/cert/dns/manual"
	"github.com/kite365/idcd/lib/cert/dns/route53"
	"github.com/kite365/idcd/lib/cert/vault"
	"github.com/kite365/idcd/lib/cert/vault/alikms"
	"github.com/kite365/idcd/lib/cert/vault/awskms"
	"github.com/kite365/idcd/lib/cert/vault/envmaster"
	"github.com/kite365/idcd/lib/cert/vault/hashivault"
	"github.com/kite365/idcd/lib/shared/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("cert-worker: load config: " + err.Error())
	}
	log := logger.New(cfg.Env)
	log.Info("cert-worker starting", "env", cfg.Env, "redis", cfg.RedisAddr, "le_env", cfg.LEEnv)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("cert-worker: pgx pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = rdb.Close() }()

	vlt, err := buildVault(cfg)
	if err != nil {
		log.Error("cert-worker: vault init", "backend", cfg.VaultBackend, "err", err)
		os.Exit(1)
	}
	log.Info("vault wired", "backend", cfg.VaultBackend, "key_id", vlt.KeyID())

	reg := dns.NewRegistry()
	for _, p := range []dns.Provider{
		cloudflare.New(cloudflare.Config{}),
		manual.New(manual.Config{}),
		aliyun.New(aliyun.Config{}),
		dnspod.New(dnspod.Config{}),
		route53.New(route53.Config{}),
		gcloud.New(gcloud.Config{}),
	} {
		if err := reg.Register(p); err != nil {
			log.Error("cert-worker: register dns provider", "kind", p.Kind(), "err", err)
			os.Exit(1)
		}
	}

	// S2: multi-CA registry. Let's Encrypt is the always-on default;
	// ZeroSSL + Buypass register only when their env vars are set so
	// dev / staging deploys without those secrets keep working.
	leCA := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.Env(cfg.LEEnv)})
	var extras []ca.AcmeCA
	if cfg.ZeroSSLEABKID != "" && cfg.ZeroSSLEABHMACKey != "" {
		extras = append(extras, zerossl.New(zerossl.Config{
			EABKID:     cfg.ZeroSSLEABKID,
			EABHMACKey: cfg.ZeroSSLEABHMACKey,
		}))
	}
	if cfg.BuypassEnv != "" {
		extras = append(extras, buypass.New(buypass.Config{Env: buypass.Env(cfg.BuypassEnv)}))
	}
	router := service.NewRouter(leCA, extras...)
	repos := repo.New(pool)

	// Quota-based fallover: when default CA's usage exceeds 70%, route
	// new orders to the first registered secondary (preference order:
	// zerossl → buypass). Secondary unset / unregistered keeps default.
	if secondary := pickSecondaryCA(router); secondary != "" {
		router = router.
			WithSecondary(secondary).
			WithQuota(service.NewRepoQuotaChecker(repos.Orders, repos.Certs, nil)).
			WithLogger(log)
		log.Info("ca quota router wired", "default", "lets-encrypt", "secondary", secondary)
	}
	log.Info("ca router wired", "cas", router.Names())

	// Long-lived ACME account key persisted in cert.acme_accounts. The
	// first invocation for a (CA, env) pair generates + inserts; later
	// invocations decrypt and reuse the same key so the CA sees a stable
	// account identity across restarts.
	acctMgr := service.NewAccountManager(repos, vlt)
	accountKey, err := acctMgr.GetOrCreate(ctx, "lets-encrypt", cfg.LEEnv)
	if err != nil {
		log.Error("cert-worker: acme account key", "err", err)
		os.Exit(1)
	}

	svc := service.New(service.Config{
		Repos:        repos,
		Redis:        rdb,
		Vault:        vlt,
		DNSReg:       reg,
		Router:       router,
		AccountKey:   accountKey,
		AccountEmail: cfg.AccountEmail,
		Logger:       log,
	})

	// Subscribe to manual-TXT readiness notifications published by the
	// API server. Runs concurrently with the order consumer; both stop
	// on ctx cancel.
	go func() {
		if err := svc.RunManualReadySubscriber(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("cert-worker: manual ready subscriber stopped", "err", err)
		}
	}()

	// S1 W4: Notification watcher. Polls cert.order_events /
	// cert.certs / cert.renewal_jobs and emits structured events on the
	// `cert:notifications` Redis Stream so the (S2) notifier service can
	// dispatch per-channel. Runs in the worker process — the renewer cmd
	// is purely a scheduler and does not need a duplicate goroutine.
	notifyWatcher := service.NewNotificationWatcher(repos, rdb,
		service.WithNotificationPool(pool),
		service.WithNotificationLogger(log),
	)
	go func() {
		if err := notifyWatcher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("cert-worker: notification watcher stopped", "err", err)
		}
	}()

	log.Info("cert-worker consuming", "stream", service.DefaultStream, "group", service.DefaultGroup)
	if err := svc.RunConsumer(ctx); err != nil {
		log.Error("cert-worker: consumer", "err", err)
		os.Exit(1)
	}
	log.Info("cert-worker stopped")
}

// pickSecondaryCA returns the first registered fallover CA (preference
// order: zerossl → buypass), or "" when no candidate is registered.
// The Router quota path is wired only when a candidate exists.
func pickSecondaryCA(r *service.Router) string {
	names := r.Names()
	for _, want := range []string{"zerossl", "buypass"} {
		for _, n := range names {
			if n == want {
				return want
			}
		}
	}
	return ""
}

// buildVault picks the vault.Vault implementation per Config.VaultBackend.
func buildVault(cfg *config.Config) (vault.Vault, error) {
	switch cfg.VaultBackend {
	case config.VaultBackendAliKMS:
		return alikms.New(alikms.Config{
			RegionID:        cfg.AliKMSRegionID,
			AccessKeyID:     cfg.AliKMSAccessKeyID,
			AccessKeySecret: cfg.AliKMSAccessKeySecret,
			KeyID:           cfg.AliKMSKeyID,
		})
	case config.VaultBackendAWSKMS:
		return awskms.New(awskms.Config{
			Region:          cfg.AWSKMSRegion,
			AccessKeyID:     cfg.AWSKMSAccessKeyID,
			SecretAccessKey: cfg.AWSKMSSecretAccessKey,
			KeyID:           cfg.AWSKMSKeyID,
		})
	case config.VaultBackendHashiVault:
		return hashivault.New(hashivault.Config{
			Address:   cfg.HashiVaultAddress,
			Token:     cfg.HashiVaultToken,
			Namespace: cfg.HashiVaultNamespace,
			KeyName:   cfg.HashiVaultKeyName,
			MountPath: cfg.HashiVaultMountPath,
		})
	default:
		return envmaster.NewFromEnv("CERT_MASTER_KEY")
	}
}
