package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
)

type OrganizationRepository struct {
	db *sql.DB
}

func NewOrganizationRepository(db *sql.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) Create(name, slug string, ownerID uuid.UUID) (*models.Organization, error) {
	o := &models.Organization{}
	err := r.db.QueryRow(
		`INSERT INTO organizations (name, slug, owner_id)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, slug, owner_id, created_at, updated_at`,
		name, slug, ownerID,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating organization: %w", err)
	}
	return o, nil
}

func (r *OrganizationRepository) GetByID(id uuid.UUID) (*models.Organization, error) {
	o := &models.Organization{}
	err := r.db.QueryRow(
		`SELECT id, name, slug, owner_id, created_at, updated_at
		 FROM organizations WHERE id = $1`,
		id,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting organization: %w", err)
	}
	return o, nil
}

func (r *OrganizationRepository) GetBySlug(slug string) (*models.Organization, error) {
	o := &models.Organization{}
	err := r.db.QueryRow(
		`SELECT id, name, slug, owner_id, created_at, updated_at
		 FROM organizations WHERE slug = $1`,
		slug,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting organization by slug: %w", err)
	}
	return o, nil
}

func (r *OrganizationRepository) ListByUserID(userID uuid.UUID) ([]models.Organization, error) {
	rows, err := r.db.Query(
		`SELECT o.id, o.name, o.slug, o.owner_id, o.created_at, o.updated_at
		 FROM organizations o
		 INNER JOIN organization_members om ON o.id = om.organization_id
		 WHERE om.user_id = $1
		 ORDER BY o.name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing organizations: %w", err)
	}
	defer rows.Close()

	var orgs []models.Organization
	for rows.Next() {
		var o models.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning organization: %w", err)
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

func (r *OrganizationRepository) Update(id uuid.UUID, name string) (*models.Organization, error) {
	o := &models.Organization{}
	err := r.db.QueryRow(
		`UPDATE organizations SET name = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, name, slug, owner_id, created_at, updated_at`,
		id, name,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating organization: %w", err)
	}
	return o, nil
}

func (r *OrganizationRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM organizations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting organization: %w", err)
	}
	return nil
}

// Members

func (r *OrganizationRepository) AddMember(orgID, userID uuid.UUID, role string) (*models.OrgMember, error) {
	m := &models.OrgMember{}
	err := r.db.QueryRow(
		`INSERT INTO organization_members (organization_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
		 RETURNING id, organization_id, user_id, role, created_at, updated_at`,
		orgID, userID, role,
	).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("adding organization member: %w", err)
	}
	return m, nil
}

func (r *OrganizationRepository) GetMember(orgID, userID uuid.UUID) (*models.OrgMember, error) {
	m := &models.OrgMember{}
	err := r.db.QueryRow(
		`SELECT id, organization_id, user_id, role, created_at, updated_at
		 FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting organization member: %w", err)
	}
	return m, nil
}

func (r *OrganizationRepository) ListMembers(orgID uuid.UUID) ([]models.OrgMember, error) {
	rows, err := r.db.Query(
		`SELECT id, organization_id, user_id, role, created_at, updated_at
		 FROM organization_members WHERE organization_id = $1
		 ORDER BY created_at`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing organization members: %w", err)
	}
	defer rows.Close()

	var members []models.OrgMember
	for rows.Next() {
		var m models.OrgMember
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning organization member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *OrganizationRepository) UpdateMemberRole(orgID, userID uuid.UUID, role string) (*models.OrgMember, error) {
	m := &models.OrgMember{}
	err := r.db.QueryRow(
		`UPDATE organization_members SET role = $3, updated_at = NOW()
		 WHERE organization_id = $1 AND user_id = $2
		 RETURNING id, organization_id, user_id, role, created_at, updated_at`,
		orgID, userID, role,
	).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating member role: %w", err)
	}
	return m, nil
}

func (r *OrganizationRepository) RemoveMember(orgID, userID uuid.UUID) error {
	_, err := r.db.Exec(
		`DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	)
	if err != nil {
		return fmt.Errorf("removing organization member: %w", err)
	}
	return nil
}

func (r *OrganizationRepository) ListProjectsByOrg(orgID uuid.UUID) ([]models.Project, error) {
	rows, err := r.db.Query(
		`SELECT id, name, description, owner_id, encrypted_dek, dek_nonce, created_at, updated_at
		 FROM projects WHERE organization_id = $1 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing organization projects: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.EncryptedDEK, &p.DEKNonce, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *OrganizationRepository) AssignProjectToOrg(projectID, orgID uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE projects SET organization_id = $2, updated_at = NOW() WHERE id = $1`,
		projectID, orgID,
	)
	if err != nil {
		return fmt.Errorf("assigning project to organization: %w", err)
	}
	return nil
}
