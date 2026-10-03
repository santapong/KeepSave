package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

// Cascade joins local run/grant invalidation to an offboarding transaction.
// Its implementation must not acquire earlier authority locks or call providers.
type Cascade interface {
	OffboardTx(context.Context, *sql.Tx, uuid.UUID, uuid.UUID) (int, error)
}

func (s *Service) EnableCascade(c Cascade) { s.cascade = c }

type Invitation struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
}

func validRole(role string) bool {
	return role == "viewer" || role == "editor" || role == "promoter" || role == "admin"
}
func lockMemberStates(ctx context.Context, tx *sql.Tx, org uuid.UUID, users []uuid.UUID) error {
	set := map[uuid.UUID]bool{}
	for _, id := range users {
		set[id] = true
	}
	ordered := make([]uuid.UUID, 0, len(set))
	for id := range set {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	for _, id := range ordered {
		// A persistent inactive state also prevents a rejoin from reviving old epochs.
		if _, e := tx.ExecContext(ctx, `INSERT INTO member_authority_state(organization_id,user_id,active,epoch) VALUES($1,$2,FALSE,1) ON CONFLICT DO NOTHING`, org, id); e != nil {
			return e
		}
		var epoch int64
		if e := tx.QueryRowContext(ctx, `SELECT epoch FROM member_authority_state WHERE organization_id=$1 AND user_id=$2 FOR UPDATE`, org, id).Scan(&epoch); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) orgActor(ctx context.Context, tx *sql.Tx, p policy.Principal, org uuid.UUID, targets []uuid.UUID, recent bool) error {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return ErrDenied
	}
	if err := s.guard.LockSubjects(ctx, tx, append([]uuid.UUID{p.SubjectID}, targets...), true); err != nil {
		return ErrDenied
	}
	var found uuid.UUID
	if err := tx.QueryRowContext(ctx, `SELECT id FROM organizations WHERE id=$1 FOR SHARE`, org).Scan(&found); err != nil {
		return ErrDenied
	}
	if err := lockMemberStates(ctx, tx, org, append([]uuid.UUID{p.SubjectID}, targets...)); err != nil {
		return err
	}
	if err := s.guard.RequireOrg(ctx, tx, p, org, "admin"); err != nil {
		return ErrDenied
	}
	if recent {
		if err := s.guard.RequireSession(ctx, tx, p, true); err != nil {
			return ErrDenied
		}
	}
	return nil
}
func (s *Service) CreateInvitation(ctx context.Context, p policy.Principal, org uuid.UUID, target *uuid.UUID, contact, role string) (Invitation, error) {
	if !validRole(role) {
		return Invitation{}, ErrInvalid
	}
	contact, err := validContact(contact)
	if err != nil {
		return Invitation{}, err
	}
	var result Invitation
	err = s.transaction(ctx, func(tx *sql.Tx) error {
		targets := []uuid.UUID{}
		if target != nil {
			targets = append(targets, *target)
		}
		if e := s.orgActor(ctx, tx, p, org, targets, true); e != nil {
			return e
		}
		if target != nil {
			var count int
			if e := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=$1`, *target).Scan(&count); e != nil {
				return e
			}
			if count != 1 {
				return ErrInvalid
			}
		}
		proof, e := s.createProof(ctx, tx, p.SubjectID, p.SessionID, "invitation", contact)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO identity_invitations(id,organization_id,inviter_id,target_user_id,intended_role) VALUES($1,$2,$3,$4,$5)`, proof.ID, org, p.SubjectID, target, role); e != nil {
			return e
		}
		if e = s.event(ctx, tx, p.SubjectID, "identity.invitation_created", map[string]string{"invitation_id": proof.ID.String(), "organization_id": org.String()}); e != nil {
			return e
		}
		result = Invitation{ID: proof.ID, OrganizationID: org, Role: role, Status: "pending"}
		return nil
	})
	return result, err
}
func (s *Service) RevokeInvitation(ctx context.Context, p policy.Principal, org, id uuid.UUID) error {
	return s.transaction(ctx, func(tx *sql.Tx) error {
		if e := s.orgActor(ctx, tx, p, org, nil, false); e != nil {
			return e
		}
		var exists uuid.UUID
		if e := tx.QueryRowContext(ctx, `SELECT id FROM identity_invitations WHERE id=$1 AND organization_id=$2 FOR UPDATE`, id, org).Scan(&exists); e != nil {
			return ErrDenied
		}
		if _, e := tx.ExecContext(ctx, `UPDATE identity_invitations SET revoked=TRUE WHERE id=$1`, id); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, `UPDATE identity_proofs SET revoked=TRUE WHERE id=$1`, id); e != nil {
			return e
		}
		return s.event(ctx, tx, p.SubjectID, "identity.invitation_revoked", map[string]string{"invitation_id": id.String(), "organization_id": org.String()})
	})
}
func (s *Service) AcceptInvitation(ctx context.Context, p policy.Principal, id uuid.UUID, raw string) error {
	if id == uuid.Nil || len(raw) != 43 {
		return ErrProof
	}
	var rejected bool
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		var org, inviter uuid.UUID
		var target uuid.NullUUID
		var role string
		if e := tx.QueryRowContext(ctx, `SELECT organization_id,inviter_id,target_user_id,intended_role FROM identity_invitations WHERE id=$1`, id).Scan(&org, &inviter, &target, &role); e != nil {
			return ErrProof
		}
		if p.Kind != policy.Human || p.ActorID != p.SubjectID || p.SubjectID == uuid.Nil {
			return ErrDenied
		}
		if e := s.guard.LockSubjects(ctx, tx, []uuid.UUID{p.SubjectID, inviter}, true); e != nil {
			return ErrDenied
		}
		var found uuid.UUID
		if e := tx.QueryRowContext(ctx, `SELECT id FROM organizations WHERE id=$1 FOR SHARE`, org).Scan(&found); e != nil {
			return ErrProof
		}
		if e := lockMemberStates(ctx, tx, org, []uuid.UUID{p.SubjectID, inviter}); e != nil {
			return e
		}
		// RequireOrg reads current inviter membership, not the role at invitation time.
		row, e := loadProof(ctx, tx, id, false)
		if e != nil {
			return ErrProof
		}
		inviterPrincipal := policy.Principal{Kind: policy.Human, SubjectID: inviter, ActorID: inviter, SessionID: row.SessionID.UUID}
		if e = s.guard.RequireOrg(ctx, tx, inviterPrincipal, org, "admin"); e != nil {
			return ErrProof
		}
		if e = s.guard.RequireSession(ctx, tx, p, false); e != nil {
			return e
		}
		var accepted uuid.NullUUID
		var revoked bool
		if e = tx.QueryRowContext(ctx, `SELECT accepted_by,revoked FROM identity_invitations WHERE id=$1 AND organization_id=$2 AND inviter_id=$3 FOR UPDATE`, id, org, inviter).Scan(&accepted, &revoked); e != nil {
			return ErrProof
		}
		if accepted.Valid || revoked {
			return ErrProof
		}
		row, e = loadProof(ctx, tx, id, true)
		if e != nil {
			return ErrProof
		}
		if row.Purpose != "invitation" || row.UserID != inviter || row.Consumed || row.Revoked || row.Attempts >= 5 || !row.Expires.After(time.Now()) {
			return ErrProof
		}
		if subtle.ConstantTimeCompare([]byte(hashProof(raw)), []byte(row.Hash)) != 1 {
			rejected = true
			_, e = tx.ExecContext(ctx, `UPDATE identity_proofs SET attempts=attempts+1,revoked=(attempts+1>=5) WHERE id=$1`, id)
			if e != nil {
				return e
			}
			return s.event(ctx, tx, inviter, "identity.proof_rejected", map[string]string{"proof_id": id.String(), "purpose": "invitation"})
		}
		if target.Valid {
			if target.UUID != p.SubjectID {
				return ErrProof
			}
		} else {
			var count int
			if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_verified_contacts WHERE user_id=$1 AND contact=$2 AND revoked_at IS NULL`, p.SubjectID, row.Contact).Scan(&count); e != nil {
				return e
			}
			if count != 1 {
				return ErrProof
			}
		}
		var owner uuid.UUID
		if e = tx.QueryRowContext(ctx, `SELECT owner_id FROM organizations WHERE id=$1`, org).Scan(&owner); e != nil {
			return e
		}
		if owner == p.SubjectID && role != "admin" {
			return ErrDenied
		}
		// Existing membership is never silently demoted/upgraded by consuming an invite.
		var existing string
		e = tx.QueryRowContext(ctx, `SELECT role FROM organization_members WHERE organization_id=$1 AND user_id=$2`, org, p.SubjectID).Scan(&existing)
		if e == nil {
			return ErrConflict
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO organization_members(id,organization_id,user_id,role) VALUES($1,$2,$3,$4)`, uuid.New(), org, p.SubjectID, role); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE identity_invitations SET accepted_by=$1,accepted_at=NOW() WHERE id=$2`, p.SubjectID, id); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE identity_proofs SET consumed=TRUE,consumed_at=NOW() WHERE id=$1`, id); e != nil {
			return e
		}
		return s.event(ctx, tx, p.SubjectID, "identity.invitation_accepted", map[string]string{"invitation_id": id.String(), "organization_id": org.String(), "user_id": p.SubjectID.String()})
	})
	if err == nil && rejected {
		return ErrProof
	}
	return err
}

type Offboarding struct {
	ID               uuid.UUID             `json:"id,omitempty"`
	PreviewID        uuid.UUID             `json:"preview_id,omitempty"`
	PreviewExpires   time.Time             `json:"preview_expires_at,omitempty"`
	OrganizationID   uuid.UUID             `json:"organization_id"`
	UserID           uuid.UUID             `json:"user_id"`
	Epoch            int64                 `json:"authority_epoch"`
	Keys             int                   `json:"keys"`
	Leases           int                   `json:"leases"`
	Runs             int                   `json:"runs"`
	Status           string                `json:"status"`
	OwnedProjects    []ProjectOwnership    `json:"projects_requiring_reassignment"`
	OwnedCredentials []CredentialOwnership `json:"credentials_requiring_reassignment"`
}

func (s *Service) offboardLocks(ctx context.Context, tx *sql.Tx, p policy.Principal, org, target uuid.UUID, exclusive bool) error {
	if p.Kind != policy.Human || p.SubjectID != p.ActorID || p.SubjectID == uuid.Nil {
		return ErrDenied
	}
	if e := s.guard.LockSubjects(ctx, tx, []uuid.UUID{p.SubjectID, target}, exclusive); e != nil {
		return e
	}
	mode := " FOR SHARE"
	if exclusive {
		mode = " FOR UPDATE"
	}
	var owner uuid.UUID
	if e := tx.QueryRowContext(ctx, `SELECT owner_id FROM organizations WHERE id=$1`+mode, org).Scan(&owner); e != nil {
		return ErrDenied
	}
	if owner == target {
		return ErrDenied
	}
	// Projects precede membership/session/grant locks. All rows are sorted.
	rows, e := tx.QueryContext(ctx, `SELECT id FROM projects WHERE organization_id=$1 ORDER BY id`+mode, org)
	if e != nil {
		return e
	}
	for rows.Next() {
		var id uuid.UUID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if exclusive {
		if e = lockMemberStates(ctx, tx, org, []uuid.UUID{p.SubjectID, target}); e != nil {
			return e
		}
	} else {
		ids := []uuid.UUID{p.SubjectID, target}
		sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
		for _, id := range ids {
			var active bool
			if e = tx.QueryRowContext(ctx, `SELECT active FROM member_authority_state WHERE organization_id=$1 AND user_id=$2 FOR SHARE`, org, id).Scan(&active); e != nil || !active {
				return ErrDenied
			}
		}
	}
	if e = s.guard.RequireOrg(ctx, tx, p, org, "admin"); e != nil {
		return ErrDenied
	}
	return nil
}
func offboardingInventory(ctx context.Context, tx *sql.Tx, org, target uuid.UUID) (Offboarding, error) {
	result := Offboarding{OrganizationID: org, UserID: target, Status: "preview"}
	var active bool
	if e := tx.QueryRowContext(ctx, `SELECT active,epoch FROM member_authority_state WHERE organization_id=$1 AND user_id=$2`, org, target).Scan(&active, &result.Epoch); e != nil || !active {
		return result, ErrDenied
	}
	if e := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.organization_id=$1 AND k.user_id=$2`, org, target).Scan(&result.Keys); e != nil {
		return result, e
	}
	if e := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM secret_leases l JOIN api_keys k ON k.id=l.api_key_id JOIN projects p ON p.id=l.project_id WHERE p.organization_id=$1 AND k.user_id=$2 AND NOT l.revoked`, org, target).Scan(&result.Leases); e != nil {
		return result, e
	}
	if e := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+offboardingRunSelection+`) affected_runs`, org, target).Scan(&result.Runs); e != nil {
		return result, e
	}
	var e error
	result.OwnedProjects, e = ownedProjects(ctx, tx, org, target)
	if e != nil {
		return result, e
	}
	result.OwnedCredentials, e = ownedCredentials(ctx, tx, org, target)
	return result, e
}
func (s *Service) PreviewOffboarding(ctx context.Context, p policy.Principal, org, target uuid.UUID) (Offboarding, error) {
	var result Offboarding
	// Counts, ownership and the private impact digest must describe one snapshot.
	// Shared admission can create child runs while these shared locks are held.
	err := s.transactionOptions(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(tx *sql.Tx) error {
		if e := s.offboardLocks(ctx, tx, p, org, target, false); e != nil {
			return e
		}
		var e error
		result, e = offboardingInventory(ctx, tx, org, target)
		if e != nil {
			return e
		}
		impact, e := offboardingImpact(ctx, tx, org, target)
		if e != nil {
			return e
		}
		result.PreviewID = uuid.New()
		result.PreviewExpires = time.Now().UTC().Add(5 * time.Minute)
		if _, e = tx.ExecContext(ctx, `INSERT INTO identity_offboarding_previews(id,organization_id,actor_id,target_user_id,authority_epoch,impact_digest,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, result.PreviewID, org, p.SubjectID, target, result.Epoch, impact, result.PreviewExpires); e != nil {
			return e
		}
		return s.event(ctx, tx, p.SubjectID, "identity.offboarding_preview_created", map[string]string{"preview_id": result.PreviewID.String(), "organization_id": org.String(), "user_id": target.String()})
	})
	return result, err
}
func (s *Service) Offboard(ctx context.Context, p policy.Principal, org, target uuid.UUID, expected int64, key string) (Offboarding, error) {
	return s.offboard(ctx, p, org, target, expected, key, uuid.Nil)
}
func (s *Service) offboard(ctx context.Context, p policy.Principal, org, target uuid.UUID, expected int64, key string, preview uuid.UUID) (Offboarding, error) {
	if len(key) < 1 || len(key) > 128 || expected < 1 {
		return Offboarding{}, ErrInvalid
	}
	digest := hashProof(fmt.Sprintf("%s/%s/%d/%s", org, target, expected, preview))
	var result Offboarding
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := s.offboardLocks(ctx, tx, p, org, target, true); e != nil {
			return e
		}
		var previous string
		var encoded []byte
		e := tx.QueryRowContext(ctx, `SELECT request_digest,summary FROM identity_offboarding_receipts WHERE organization_id=$1 AND actor_id=$2 AND request_key=$3`, org, p.SubjectID, key).Scan(&previous, &encoded)
		if e == nil {
			if previous != digest {
				return ErrConflict
			}
			return json.Unmarshal(encoded, &result)
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		result, e = offboardingInventory(ctx, tx, org, target)
		if e != nil {
			return e
		}
		if result.Epoch != expected {
			return ErrConflict
		}
		if preview != uuid.Nil {
			var savedImpact string
			var expires time.Time
			if e = tx.QueryRowContext(ctx, `SELECT impact_digest,expires_at FROM identity_offboarding_previews WHERE id=$1 AND organization_id=$2 AND actor_id=$3 AND target_user_id=$4 AND authority_epoch=$5 AND consumed_at IS NULL FOR UPDATE`, preview, org, p.SubjectID, target, expected).Scan(&savedImpact, &expires); e != nil || !expires.After(time.Now()) {
				return ErrConflict
			}
			impact, err := offboardingImpact(ctx, tx, org, target)
			if err != nil {
				return err
			}
			if savedImpact != impact {
				return ErrConflict
			}
			if _, e = tx.ExecContext(ctx, `UPDATE identity_offboarding_previews SET consumed_at=NOW() WHERE id=$1`, preview); e != nil {
				return e
			}
			result.PreviewID = preview
			result.PreviewExpires = expires
		}
		if s.cascade == nil {
			var requiresCascade bool
			if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tool_grants g JOIN projects p ON p.id=g.project_id WHERE p.organization_id=$1 AND g.revoked_at IS NULL AND `+offboardingGrantPredicate+`) OR EXISTS(`+offboardingRunSelection+`) OR EXISTS(SELECT 1 FROM tool_connections c JOIN projects p ON p.id=c.project_id WHERE p.organization_id=$1 AND c.created_by=$2 AND c.revoked_at IS NULL) OR EXISTS(SELECT 1 FROM tool_workloads w JOIN projects p ON p.id=w.project_id WHERE p.organization_id=$1 AND w.created_by=$2 AND w.revoked_at IS NULL)`, org, target).Scan(&requiresCascade); e != nil {
				return e
			}
			if requiresCascade {
				return ErrUnavailable
			}
		}
		if _, e = tx.ExecContext(ctx, `DELETE FROM organization_members WHERE organization_id=$1 AND user_id=$2`, org, target); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `DELETE FROM api_keys k USING projects p WHERE p.id=k.project_id AND p.organization_id=$1 AND k.user_id=$2`, org, target); e != nil {
			return e
		}
		// Run/attempt invalidation precedes approval locks in the shared order.
		if s.cascade != nil {
			result.Runs, e = s.cascade.OffboardTx(ctx, tx, org, target)
			if e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(ctx, `UPDATE promotion_requests pr SET status='cancelled',approved_by=NULL FROM projects p WHERE p.id=pr.project_id AND p.organization_id=$1 AND (pr.requested_by=$2 OR pr.approved_by=$2) AND pr.status IN ('pending','approved')`, org, target); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE identity_invitations i SET revoked=TRUE FROM identity_proofs proof WHERE proof.id=i.id AND i.organization_id=$1 AND i.accepted_by IS NULL AND (i.inviter_id=$2 OR i.target_user_id=$2 OR EXISTS(SELECT 1 FROM identity_verified_contacts c WHERE c.user_id=$2 AND c.contact=proof.contact AND c.revoked_at IS NULL))`, org, target); e != nil {
			return e
		}
		result.ID = uuid.New()
		result.Epoch++
		result.Status = "locally_revoked"
		encoded, e = json.Marshal(result)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO identity_offboarding_receipts(id,organization_id,actor_id,target_user_id,request_key,request_digest,authority_epoch,summary) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, result.ID, org, p.SubjectID, target, key, digest, result.Epoch, encoded); e != nil {
			return e
		}
		return s.event(ctx, tx, p.SubjectID, "identity.member_offboarded", map[string]string{"receipt_id": result.ID.String(), "organization_id": org.String(), "user_id": target.String()})
	})
	return result, err
}
