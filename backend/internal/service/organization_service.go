package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/jobs"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

var slugRegex = regexp.MustCompile(`[^a-z0-9-]+`)
var workspaceRequestKey = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

var ErrOrganizationConflict = errors.New("workspace operation conflicts with current state")
var ErrWorkspaceInput = errors.New("invalid workspace request")

type OrganizationService struct {
	orgRepo   *repository.OrganizationRepository
	auditRepo *repository.AuditRepository
}

func NewOrganizationService(orgRepo *repository.OrganizationRepository, auditRepo *repository.AuditRepository) *OrganizationService {
	return &OrganizationService{orgRepo: orgRepo, auditRepo: auditRepo}
}

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = slugRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "org"
	}
	return slug
}

func (s *OrganizationService) Create(name string, ownerID uuid.UUID, ipAddr string) (*models.Organization, error) {
	return s.CreateWorkspace(name, ownerID, ipAddr, "")
}

// CreateWorkspace makes onboarding retryable without duplicating workspaces.
// Legacy callers may omit the key; the onboarding client must keep it for retries.
func (s *OrganizationService) CreateWorkspace(name string, ownerID uuid.UUID, ipAddr, requestKey string) (*models.Organization, error) {
	name = strings.TrimSpace(name)
	if !validWorkspaceName(name) || ownerID == uuid.Nil || (requestKey != "" && !workspaceRequestKey.MatchString(requestKey)) {
		return nil, ErrWorkspaceInput
	}
	digestBytes := sha256.Sum256([]byte(name))
	digest := hex.EncodeToString(digestBytes[:])
	var result *models.Organization
	err := s.orgRepo.WithTx(func(tx *sql.Tx) error {
		// Serializes caller-scoped idempotency before any workspace is inserted.
		if err := s.orgRepo.LockUserTx(tx, ownerID); err != nil {
			return err
		}
		if requestKey != "" {
			previous, id, err := s.orgRepo.GetWorkspaceRequestTx(tx, ownerID, requestKey)
			if err == nil {
				if previous != digest {
					return ErrOrganizationConflict
				}
				result, err = s.orgRepo.GetByIDTx(tx, id)
				if errors.Is(err, sql.ErrNoRows) {
					return ErrOrganizationConflict // deleted workspace is never recreated
				}
				return err
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		id := uuid.New()
		prefix := generateSlug(name)
		if len(prefix) > 180 {
			prefix = strings.TrimRight(prefix[:180], "-")
		}
		var err error
		result, err = s.orgRepo.CreateTx(tx, id, name, prefix+"-"+id.String(), ownerID)
		if err != nil {
			return err
		}
		if _, err = s.orgRepo.AddMemberTx(tx, result.ID, ownerID, "admin"); err != nil {
			return err
		}
		if requestKey != "" {
			if err = s.orgRepo.CreateWorkspaceRequestTx(tx, ownerID, requestKey, digest, result.ID); err != nil {
				return err
			}
		}
		return s.recordTx(tx, ownerID, nil, "org.created", models.JSONMap{"organization_id": result.ID.String(), "name": name}, ipAddr)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OrganizationService) GetByID(id, userID uuid.UUID) (*models.Organization, error) {
	// Verify membership
	if _, err := s.orgRepo.GetMember(id, userID); err != nil {
		return nil, fmt.Errorf("not a member of this organization")
	}

	org, err := s.orgRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("getting organization: %w", err)
	}
	return org, nil
}

func (s *OrganizationService) List(userID uuid.UUID) ([]models.Organization, error) {
	return s.orgRepo.ListByUserID(userID)
}

func (s *OrganizationService) Update(id, userID uuid.UUID, name, ipAddr string) (*models.Organization, error) {
	name = strings.TrimSpace(name)
	if !validWorkspaceName(name) {
		return nil, ErrWorkspaceInput
	}
	var org *models.Organization
	err := s.orgRepo.WithTx(func(tx *sql.Tx) error {
		if _, err := s.requireRoleTx(tx, id, userID, "admin"); err != nil {
			return err
		}
		var err error
		org, err = s.orgRepo.UpdateTx(tx, id, name)
		if err != nil {
			return err
		}
		return s.recordTx(tx, userID, nil, "org.updated", models.JSONMap{"organization_id": id.String(), "name": name}, ipAddr)
	})
	return org, err
}

func (s *OrganizationService) Delete(id, userID uuid.UUID, ipAddr string) error {
	return s.orgRepo.WithTx(func(tx *sql.Tx) error {
		org, err := s.requireRoleTx(tx, id, userID, "admin")
		if err != nil {
			return err
		}
		if org.OwnerID != userID {
			return ErrOrgAccessDenied
		}
		count, err := s.orgRepo.CountProjectsTx(tx, id)
		if err != nil {
			return err
		}
		if count != 0 {
			return ErrOrganizationConflict
		}
		if err = s.orgRepo.DeleteTx(tx, id); err != nil {
			return err
		}
		return s.recordTx(tx, userID, nil, "org.deleted", models.JSONMap{"organization_id": id.String(), "name": org.Name}, ipAddr)
	})
}

func (s *OrganizationService) AddMember(orgID, userID, targetUserID uuid.UUID, role, ipAddr string) (*models.OrgMember, error) {
	if !isValidRole(role) {
		return nil, ErrWorkspaceInput
	}
	var member *models.OrgMember
	err := s.orgRepo.WithTx(func(tx *sql.Tx) error {
		org, err := s.requireRoleTx(tx, orgID, userID, "admin")
		if err != nil {
			return err
		}
		if org.OwnerID == targetUserID && role != "admin" {
			return ErrOrgAccessDenied
		}
		member, err = s.orgRepo.AddMemberTx(tx, orgID, targetUserID, role)
		if err != nil {
			return err
		}
		return s.recordTx(tx, userID, nil, "org.member_added", models.JSONMap{"organization_id": orgID.String(), "target_user_id": targetUserID.String(), "role": role}, ipAddr)
	})
	return member, err
}

func (s *OrganizationService) ListMembers(orgID, userID uuid.UUID) ([]models.OrgMember, error) {
	if _, err := s.orgRepo.GetMember(orgID, userID); err != nil {
		return nil, fmt.Errorf("not a member of this organization")
	}
	return s.orgRepo.ListMembers(orgID)
}

func (s *OrganizationService) UpdateMemberRole(orgID, userID, targetUserID uuid.UUID, role, ipAddr string) (*models.OrgMember, error) {
	if !isValidRole(role) {
		return nil, ErrWorkspaceInput
	}
	var member *models.OrgMember
	err := s.orgRepo.WithTx(func(tx *sql.Tx) error {
		org, err := s.requireRoleTx(tx, orgID, userID, "admin")
		if err != nil {
			return err
		}
		if org.OwnerID == targetUserID && role != "admin" {
			return ErrOrgAccessDenied
		}
		member, err = s.orgRepo.UpdateMemberRoleTx(tx, orgID, targetUserID, role)
		if err != nil {
			return err
		}
		return s.recordTx(tx, userID, nil, "org.member_role_updated", models.JSONMap{"organization_id": orgID.String(), "target_user_id": targetUserID.String(), "role": role}, ipAddr)
	})
	return member, err
}

func (s *OrganizationService) RemoveMember(orgID, userID, targetUserID uuid.UUID, ipAddr string) error {
	return s.orgRepo.WithTx(func(tx *sql.Tx) error {
		org, err := s.requireRoleTx(tx, orgID, userID, "admin")
		if err != nil {
			return err
		}
		if org.OwnerID == targetUserID {
			return ErrOrgAccessDenied
		}
		if err = s.orgRepo.RemoveMemberTx(tx, orgID, targetUserID); err != nil {
			return err
		}
		return s.recordTx(tx, userID, nil, "org.member_removed", models.JSONMap{"organization_id": orgID.String(), "target_user_id": targetUserID.String()}, ipAddr)
	})
}

func (s *OrganizationService) AssignProject(orgID, userID, projectID uuid.UUID) error {
	return s.AssignProjectWithAudit(orgID, userID, projectID, "")
}

func (s *OrganizationService) AssignProjectWithAudit(orgID, userID, projectID uuid.UUID, ipAddr string) error {
	return s.orgRepo.WithTx(func(tx *sql.Tx) error {
		// Project first matches vault lock order and prevents concurrent reassignment.
		project, err := s.orgRepo.LockProjectAssignmentTx(tx, projectID)
		if err != nil || project.DeletedAt.Valid || project.OwnerID != userID {
			return ErrOrgAccessDenied
		}
		if _, err = s.requireRoleTx(tx, orgID, userID, "admin"); err != nil {
			return err
		}
		if project.OrganizationID.Valid {
			if project.OrganizationID.UUID == orgID {
				return nil
			}
			return ErrOrganizationConflict
		}
		if err = s.orgRepo.AssignPersonalProjectTx(tx, projectID, userID, orgID); err != nil {
			return err
		}
		revoked, err := s.orgRepo.RevokeProjectDelegationTx(tx, projectID)
		if err != nil {
			return err
		}
		return s.recordTx(tx, userID, &projectID, "org.project_assigned", models.JSONMap{"organization_id": orgID.String(), "project_id": projectID.String(), "revoked_api_keys": revoked}, ipAddr)
	})
}

func validWorkspaceName(name string) bool {
	return name != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= 255
}

func (s *OrganizationService) requireRoleTx(tx *sql.Tx, orgID, actorID uuid.UUID, role string) (*models.Organization, error) {
	org, err := s.orgRepo.LockOrganizationTx(tx, orgID)
	if err != nil {
		return nil, ErrOrgAccessDenied
	}
	member, err := s.orgRepo.GetMemberTx(tx, orgID, actorID)
	if err != nil || !hasPermission(member.Role, role) {
		return nil, ErrOrgAccessDenied
	}
	return org, nil
}

func (s *OrganizationService) recordTx(tx *sql.Tx, actor uuid.UUID, project *uuid.UUID, action string, details models.JSONMap, ipAddr string) error {
	if s.auditRepo == nil || actor == uuid.Nil {
		return ErrOrgAccessDenied
	}
	if err := s.auditRepo.CreateTx(tx, &actor, project, action, "", details, ipAddr); err != nil {
		return err
	}
	if !s.orgRepo.SupportsDurableJobs() {
		return nil // legacy dialects do not advertise the PostgreSQL jobs capability
	}
	payload, err := json.Marshal(map[string]interface{}{"actor_id": actor, "action": action, "details": details})
	if err != nil {
		return err
	}
	return jobs.EnqueueTx(context.Background(), tx, uuid.New(), "org.event", payload, "local", 5)
}

func (s *OrganizationService) ListProjects(orgID, userID uuid.UUID) ([]models.Project, error) {
	if _, err := s.orgRepo.GetMember(orgID, userID); err != nil {
		return nil, fmt.Errorf("not a member of this organization")
	}
	return s.orgRepo.ListProjectsByOrg(orgID)
}

func (s *OrganizationService) GetMemberRole(orgID, userID uuid.UUID) (string, error) {
	member, err := s.orgRepo.GetMember(orgID, userID)
	if err != nil {
		return "", fmt.Errorf("not a member of this organization")
	}
	return member.Role, nil
}

// ErrOrgAccessDenied is returned when a caller is not a member of an
// organization, or is a member but lacks the required role. API handlers map
// it to 403.
var ErrOrgAccessDenied = errors.New("organization access denied")

// requireOrgRole verifies userID is a member of orgID with at least
// requiredRole. It is shared by OrganizationService, SSOService and
// ComplianceService so every org-scoped mutation enforces membership the same
// way. Failures wrap ErrOrgAccessDenied so callers can map them to a 403.
func requireOrgRole(orgRepo *repository.OrganizationRepository, orgID, userID uuid.UUID, requiredRole string) error {
	member, err := orgRepo.GetMember(orgID, userID)
	if err != nil {
		return fmt.Errorf("%w: not a member of this organization", ErrOrgAccessDenied)
	}
	if !hasPermission(member.Role, requiredRole) {
		return fmt.Errorf("%w: requires %s role", ErrOrgAccessDenied, requiredRole)
	}
	return nil
}

func (s *OrganizationService) requireRole(orgID, userID uuid.UUID, requiredRole string) error {
	return requireOrgRole(s.orgRepo, orgID, userID, requiredRole)
}

func isValidRole(role string) bool {
	switch role {
	case "viewer", "editor", "admin", "promoter":
		return true
	}
	return false
}

// hasPermission checks if the user's role meets the required role level.
// Role hierarchy: admin > promoter > editor > viewer
func hasPermission(userRole, requiredRole string) bool {
	roleLevel := map[string]int{
		"viewer":   1,
		"editor":   2,
		"promoter": 3,
		"admin":    4,
	}
	userLevel, knownUser := roleLevel[userRole]
	requiredLevel, knownRequired := roleLevel[requiredRole]
	return knownUser && knownRequired && userLevel >= requiredLevel
}

// CheckProjectAccess verifies a user has the required role for a project within an org.
func (s *OrganizationService) CheckProjectAccess(orgID, userID uuid.UUID, requiredRole string) error {
	return s.requireRole(orgID, userID, requiredRole)
}
