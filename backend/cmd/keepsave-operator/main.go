// This command runs only on the trusted control host. It is never an HTTP API.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/crypto/keyprovider"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"os"
	"os/user"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Operator action failed. Check the account ID, database access and audit configuration.")
		os.Exit(1)
	}
}
func run() error {
	action := flag.String("action", "", "grant or revoke")
	target := flag.String("user-id", "", "verified immutable KeepSave account ID")
	reason := flag.String("reason", "", "reviewed reason for the grant or revocation")
	flag.Parse()
	if *action != "grant" && *action != "revoke" {
		return fmt.Errorf("invalid action")
	}
	id, err := uuid.Parse(*target)
	if err != nil {
		return err
	}
	operator, err := user.Current()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, d, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	// Derive the existing audit key exactly as the API does; no new key format.
	key := cfg.MasterKey
	if cfg.KeyProvider == "vault" {
		p, e := keyprovider.NewVaultProvider(nil, cfg.VaultAddr, cfg.VaultToken, cfg.VaultKeyName, cfg.VaultCiphertext)
		if e != nil {
			return e
		}
		key, err = p.GetMasterKey(ctx)
		if err != nil {
			return err
		}
	} else if cfg.KeyProvider != "" && cfg.KeyProvider != "env" {
		return fmt.Errorf("unsupported key provider")
	}
	cs, err := crypto.NewService(key)
	if err != nil {
		return err
	}
	audit := repository.NewAuditRepository(db, d)
	audit.SetChainKey(cs.DeriveAuditChainKey())
	if broken, e := audit.VerifyChain(); e != nil || broken != nil {
		return fmt.Errorf("audit verification failed")
	}
	if err = repository.NewPlatformAdminRepository(db, d, audit).SetGrant(ctx, id, *action == "grant", operator.Username, *reason); err != nil {
		return err
	}
	fmt.Println("Operator grant updated and audited.")
	return nil
}
