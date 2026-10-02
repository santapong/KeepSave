// keepsave-vault is a trusted-host migration/recovery command, never an HTTP API.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/crypto/keyprovider"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"github.com/santapong/KeepSave/backend/migrations"
	"io"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Vault operation failed; source state was not replaced. Check the isolated target, migration and recovery material.")
		os.Exit(1)
	}
}
func run() error {
	action := flag.String("action", "", "baseline, verify or recover-isolated")
	path := flag.String("bundle", "", "external encrypted JSON bundle")
	name := flag.String("target-name", "", "explicit isolated recovered project name")
	isolated := flag.Bool("confirm-empty-isolated-database", false, "target is an isolated empty PostgreSQL database, never the live database")
	flag.Parse()
	if *action != "baseline" && *action != "verify" && *action != "recover-isolated" {
		return fmt.Errorf("invalid action")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cfg, err := config.Load()
	if err != nil {
		return err
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
		return fmt.Errorf("operator key provider unsupported")
	}
	keys, err := crypto.NewService(key)
	if err != nil {
		return err
	}
	var bundle vault.Bundle
	if *action != "baseline" {
		file, e := os.Open(*path)
		if e != nil {
			return e
		}
		defer file.Close()
		decoder := json.NewDecoder(io.LimitReader(file, 90<<20))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&bundle); e != nil {
			return e
		}
		var extra any
		if e = decoder.Decode(&extra); e != io.EOF {
			return fmt.Errorf("bundle contains extra data")
		}
		verified, e := vault.VerifyBackup(bundle, keys)
		if e != nil {
			return e
		}
		if *action == "verify" {
			return json.NewEncoder(os.Stdout).Encode(verified)
		}
	}
	if *action == "recover-isolated" && (!*isolated || *name == "") {
		return fmt.Errorf("explicit isolated target required")
	}
	db, d, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if d.DBType() != repository.DBTypePostgres {
		return fmt.Errorf("PostgreSQL required")
	}
	if *action == "recover-isolated" {
		release, e := lockFreshRecoveryTarget(ctx, db)
		if e != nil {
			return e
		}
		defer release()
	}
	if err = repository.RunMigrationsFS(db, d, migrations.FS); err != nil {
		return err
	}
	audit := repository.NewAuditRepository(db, d)
	audit.SetChainKey(keys.DeriveAuditChainKey())
	if broken, e := audit.VerifyChain(); e != nil || broken != nil {
		return fmt.Errorf("audit chain invalid")
	}
	// This command's operator owns the control host and key source. Its authority
	// is confined to stored project ownership for baseline and isolated recovery;
	// this session-less adapter is never passed to an HTTP handler.
	v := vault.New(db, keys, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: d}}).Authorize(ctx, p, a, r)
	}, audit)
	if *action == "baseline" {
		rows, e := db.QueryContext(ctx, `SELECT p.id,COALESCE(o.owner_id,p.owner_id) FROM projects p LEFT JOIN organizations o ON o.id=p.organization_id WHERE p.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM vault_projects WHERE project_id=p.id) ORDER BY p.id`)
		if e != nil {
			return e
		}
		type target struct{ id, owner uuid.UUID }
		var targets []target
		for rows.Next() {
			var item target
			if e = rows.Scan(&item.id, &item.owner); e != nil {
				rows.Close()
				return e
			}
			targets = append(targets, item)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, item := range targets {
			if e = v.Enroll(ctx, policy.Principal{Kind: policy.Human, SubjectID: item.owner, ActorID: item.owner}, item.id); e != nil {
				return e
			}
		}
		fmt.Printf("Enrolled %d projects as labeled encrypted baselines. No prior history was invented.\n", len(targets))
		return nil
	}
	var occupied int
	if err = db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM users)+(SELECT COUNT(*) FROM projects)+(SELECT COUNT(*) FROM organizations)+(SELECT COUNT(*) FROM session_tokens)+(SELECT COUNT(*) FROM api_keys)`).Scan(&occupied); err != nil {
		return err
	}
	if occupied != 0 {
		return fmt.Errorf("isolated target must be empty")
	}
	owner, err := repository.NewUserRepository(db, d).Create("isolated-recovery-"+uuid.NewString()+"@example.invalid", "!")
	if err != nil {
		return err
	}
	ps := service.NewProjectService(repository.NewProjectRepository(db, d), repository.NewEnvironmentRepository(db, d), audit, keys)
	project, err := ps.Create(*name, "Isolated vault recovery; no source authority restored", owner.ID, "")
	if err != nil {
		return err
	}
	p := policy.Principal{Kind: policy.Human, SubjectID: owner.ID, ActorID: owner.ID}
	result, err := v.RecoverToEmptyProject(ctx, p, project.ID, bundle, keys)
	if err != nil {
		return err
	}
	if broken, e := audit.VerifyChain(); e != nil || broken != nil {
		return fmt.Errorf("recovery audit invalid")
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
