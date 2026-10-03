// Trusted local outbox worker. Connector execution is intentionally absent.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auditview"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/crypto/keyprovider"
	"github.com/santapong/KeepSave/backend/internal/identity"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/runs"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Trusted worker stopped; check private storage, database, migration, and key configuration.")
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, d, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if d.DBType() != repository.DBTypePostgres {
		return fmt.Errorf("PostgreSQL required")
	}
	key := cfg.MasterKey
	if cfg.KeyProvider == "vault" {
		provider, e := keyprovider.NewVaultProvider(nil, cfg.VaultAddr, cfg.VaultToken, cfg.VaultKeyName, cfg.VaultCiphertext)
		if e != nil {
			return e
		}
		key, err = provider.GetMasterKey(ctx)
		if err != nil {
			return err
		}
	} else if cfg.KeyProvider != "env" {
		return fmt.Errorf("unsupported key source")
	}
	keys, err := crypto.NewService(key)
	if err != nil {
		return err
	}
	audit := repository.NewAuditRepository(db, d)
	audit.SetChainKey(keys.DeriveAuditChainKey())
	if broken, e := audit.VerifyChain(); e != nil || broken != nil {
		return fmt.Errorf("audit chain invalid")
	}
	v := vault.New(db, keys, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: d}}).Authorize(ctx, p, a, r)
	}, audit)
	auditViews := auditview.New(db, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: d, RequireHumanSession: true}}).Authorize(ctx, p, a, r)
	}, audit)
	toolRuns := runs.New(db, broker.New(broker.CryptoCustody{Service: keys}), func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: d, RequireHumanSession: true}}).Authorize(ctx, p, a, r)
	}, audit, runs.Flags{Enabled: cfg.Platform.RunsEnabled})
	toolRuns.LockMaintenanceSubjects = authority.Postgres(db).LockProjectMaintenance
	enabled := false
	if value := os.Getenv("KEEPSAVE_RECOVERY_VERIFIED"); value != "" {
		enabled, err = strconv.ParseBool(value)
		if err != nil {
			return err
		}
	}
	identityService := identity.New(db, identity.Config{Enabled: cfg.Platform.IdentityEnabled, PostgreSQL: true, SMTPConfigured: cfg.Platform.SMTPConfigured() && cfg.Platform.SMTPAccepted, ApplicationOrigin: cfg.Platform.ApplicationOrigin}, audit, vault.NewPayloadCustody(keys), authority.Postgres(db))
	var sender identity.Sender
	if cfg.Platform.IdentityEnabled && cfg.Platform.SMTPAccepted {
		sender, err = identity.NewSMTP(identity.SMTPConfig{Address: net.JoinHostPort(cfg.Platform.SMTPHost, cfg.Platform.SMTPPort), Hostname: cfg.Platform.SMTPHost, Username: cfg.Platform.SMTPUsername, Password: cfg.Platform.SMTPPassword, From: cfg.Platform.SMTPFrom})
		if err != nil {
			return err
		}
	}
	m := vault.Maintenance{Vault: v, Directory: os.Getenv("KEEPSAVE_BACKUP_DIRECTORY"), WorkerID: uuid.New(), RecoveryVerified: enabled}
	if err = m.Validate(); err != nil {
		return err
	}
	fmt.Println("Trusted worker started; daily encrypted backups require operator-confirmed recovery acceptance.")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastSchedule := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			if now.Sub(lastSchedule) >= time.Minute {
				if e := m.Schedule(ctx, now); e != nil {
					fmt.Fprintln(os.Stderr, "Backup scheduling failed; retry remains pending.")
				}
				if cfg.Platform.TeamVaultEnabled {
					if _, e := v.GenerateReminders(ctx, now); e != nil {
						fmt.Fprintln(os.Stderr, "Reminder checks failed; no notification success recorded.")
					}
				}
				if identityService.Enabled() {
					if _, e := identityService.SweepDeliveries(ctx); e != nil {
						fmt.Fprintln(os.Stderr, "Proof recovery failed; uncertain deliveries remain unavailable.")
					}
				}
				if e := auditViews.Cleanup(ctx); e != nil {
					fmt.Fprintln(os.Stderr, "Expired export cleanup failed; expired downloads remain denied.")
				}
				if _, e := toolRuns.ReconcileExpired(ctx); e != nil {
					fmt.Fprintln(os.Stderr, "Operation recovery failed; uncertain work will not be replayed.")
				}
				lastSchedule = now
			}
			if cfg.Platform.TeamVaultEnabled {
				if _, e := auditViews.StepPublication(ctx, m.WorkerID); e != nil {
					fmt.Fprintln(os.Stderr, "Audit export publication failed; inspect durable job status.")
				}
			}
			if sender != nil {
				if _, e := identityService.StepDelivery(ctx, m.WorkerID, sender); e != nil {
					fmt.Fprintln(os.Stderr, "Proof delivery could not be confirmed; inspect durable outcome before explicit resend.")
				}
			}
			if e := m.Step(ctx); e != nil && !errors.Is(e, jobs.ErrNoJob) {
				fmt.Fprintln(os.Stderr, "Local job failed; inspect durable status and bounded retry count.")
			}
		}
	}
}
