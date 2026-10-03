package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

// A request-scoped facade carries authenticated session evidence into legacy
// business methods without mutating the shared singleton or weakening fixtures.
func (s *OrganizationService) authorized(ctx context.Context, p policy.Principal) (*OrganizationService, error) {
	if p.Kind != policy.Human || p.SubjectID == uuid.Nil || p.ActorID != p.SubjectID {
		return nil, ErrOrgAccessDenied
	}
	copy := *s
	copy.principal = &p
	copy.requestContext = ctx
	return &copy, nil
}
func (s *OrganizationService) CreateWorkspaceAuthorized(ctx context.Context, p policy.Principal, name, ip, key string) (*models.Organization, error) {
	scoped, e := s.authorized(ctx, p)
	if e != nil {
		return nil, e
	}
	return scoped.CreateWorkspace(name, p.SubjectID, ip, key)
}
func (s *OrganizationService) UpdateAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID, name, ip string) (*models.Organization, error) {
	scoped, e := s.authorized(ctx, p)
	if e != nil {
		return nil, e
	}
	return scoped.Update(id, p.SubjectID, name, ip)
}
func (s *OrganizationService) DeleteAuthorized(ctx context.Context, p policy.Principal, id uuid.UUID, ip string) error {
	scoped, e := s.authorized(ctx, p)
	if e != nil {
		return e
	}
	return scoped.Delete(id, p.SubjectID, ip)
}
func (s *OrganizationService) AddMemberAuthorized(ctx context.Context, p policy.Principal, org, target uuid.UUID, role, ip string) (*models.OrgMember, error) {
	scoped, e := s.authorized(ctx, p)
	if e != nil {
		return nil, e
	}
	return scoped.AddMember(org, p.SubjectID, target, role, ip)
}
func (s *OrganizationService) UpdateMemberRoleAuthorized(ctx context.Context, p policy.Principal, org, target uuid.UUID, role, ip string) (*models.OrgMember, error) {
	scoped, e := s.authorized(ctx, p)
	if e != nil {
		return nil, e
	}
	return scoped.UpdateMemberRole(org, p.SubjectID, target, role, ip)
}
func (s *OrganizationService) RemoveMemberAuthorized(ctx context.Context, p policy.Principal, org, target uuid.UUID, ip string) error {
	scoped, e := s.authorized(ctx, p)
	if e != nil {
		return e
	}
	return scoped.RemoveMember(org, p.SubjectID, target, ip)
}
func (s *OrganizationService) AssignProjectAuthorized(ctx context.Context, p policy.Principal, org, project uuid.UUID, ip string) error {
	scoped, e := s.authorized(ctx, p)
	if e != nil {
		return e
	}
	return scoped.AssignProjectWithAudit(org, p.SubjectID, project, ip)
}
