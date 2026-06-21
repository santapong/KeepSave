package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

var slugRegex = regexp.MustCompile(`[^a-z0-9-]+`)

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
	if name == "" {
		return nil, fmt.Errorf("organization name is required")
	}

	slug := generateSlug(name)

	org, err := s.orgRepo.Create(name, slug, ownerID)
	if err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}

	// Add owner as admin member
	if _, err := s.orgRepo.AddMember(org.ID, ownerID, "admin"); err != nil {
		return nil, fmt.Errorf("adding owner as admin: %w", err)
	}

	emitAudit(s.auditRepo, &ownerID, nil, "org.created", "",
		models.JSONMap{"organization_id": org.ID.String(), "name": name}, ipAddr)

	return org, nil
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
	if err := s.requireRole(id, userID, "admin"); err != nil {
		return nil, err
	}
	org, err := s.orgRepo.Update(id, name)
	if err != nil {
		return nil, err
	}
	emitAudit(s.auditRepo, &userID, nil, "org.updated", "",
		models.JSONMap{"organization_id": id.String(), "name": name}, ipAddr)
	return org, nil
}

func (s *OrganizationService) Delete(id, userID uuid.UUID, ipAddr string) error {
	org, err := s.orgRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("getting organization: %w", err)
	}
	if org.OwnerID != userID {
		return fmt.Errorf("only the owner can delete the organization")
	}
	if err := s.orgRepo.Delete(id); err != nil {
		return err
	}
	emitAudit(s.auditRepo, &userID, nil, "org.deleted", "",
		models.JSONMap{"organization_id": id.String(), "name": org.Name}, ipAddr)
	return nil
}

func (s *OrganizationService) AddMember(orgID, userID, targetUserID uuid.UUID, role, ipAddr string) (*models.OrgMember, error) {
	if err := s.requireRole(orgID, userID, "admin"); err != nil {
		return nil, err
	}
	if !isValidRole(role) {
		return nil, fmt.Errorf("invalid role: %s (must be viewer, editor, admin, or promoter)", role)
	}
	member, err := s.orgRepo.AddMember(orgID, targetUserID, role)
	if err != nil {
		return nil, err
	}
	emitAudit(s.auditRepo, &userID, nil, "org.member_added", "",
		models.JSONMap{"organization_id": orgID.String(), "target_user_id": targetUserID.String(), "role": role}, ipAddr)
	return member, nil
}

func (s *OrganizationService) ListMembers(orgID, userID uuid.UUID) ([]models.OrgMember, error) {
	if _, err := s.orgRepo.GetMember(orgID, userID); err != nil {
		return nil, fmt.Errorf("not a member of this organization")
	}
	return s.orgRepo.ListMembers(orgID)
}

func (s *OrganizationService) UpdateMemberRole(orgID, userID, targetUserID uuid.UUID, role, ipAddr string) (*models.OrgMember, error) {
	if err := s.requireRole(orgID, userID, "admin"); err != nil {
		return nil, err
	}
	if !isValidRole(role) {
		return nil, fmt.Errorf("invalid role: %s", role)
	}
	member, err := s.orgRepo.UpdateMemberRole(orgID, targetUserID, role)
	if err != nil {
		return nil, err
	}
	emitAudit(s.auditRepo, &userID, nil, "org.member_role_updated", "",
		models.JSONMap{"organization_id": orgID.String(), "target_user_id": targetUserID.String(), "role": role}, ipAddr)
	return member, nil
}

func (s *OrganizationService) RemoveMember(orgID, userID, targetUserID uuid.UUID, ipAddr string) error {
	if err := s.requireRole(orgID, userID, "admin"); err != nil {
		return err
	}
	org, err := s.orgRepo.GetByID(orgID)
	if err != nil {
		return fmt.Errorf("getting organization: %w", err)
	}
	if org.OwnerID == targetUserID {
		return fmt.Errorf("cannot remove the organization owner")
	}
	if err := s.orgRepo.RemoveMember(orgID, targetUserID); err != nil {
		return err
	}
	emitAudit(s.auditRepo, &userID, nil, "org.member_removed", "",
		models.JSONMap{"organization_id": orgID.String(), "target_user_id": targetUserID.String()}, ipAddr)
	return nil
}

func (s *OrganizationService) AssignProject(orgID, userID, projectID uuid.UUID) error {
	if err := s.requireRole(orgID, userID, "admin"); err != nil {
		return err
	}
	return s.orgRepo.AssignProjectToOrg(projectID, orgID)
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

func (s *OrganizationService) requireRole(orgID, userID uuid.UUID, requiredRole string) error {
	member, err := s.orgRepo.GetMember(orgID, userID)
	if err != nil {
		return fmt.Errorf("not a member of this organization")
	}
	if !hasPermission(member.Role, requiredRole) {
		return fmt.Errorf("insufficient permissions: requires %s role", requiredRole)
	}
	return nil
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
	return roleLevel[userRole] >= roleLevel[requiredRole]
}

// CheckProjectAccess verifies a user has the required role for a project within an org.
func (s *OrganizationService) CheckProjectAccess(orgID, userID uuid.UUID, requiredRole string) error {
	return s.requireRole(orgID, userID, requiredRole)
}
