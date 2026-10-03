package runs

import (
	"encoding/json"
	"github.com/google/uuid"
)

// These public identifiers are captured when preparation is admitted. They are
// neither provider credentials nor an authorization substitute.
type runScope struct {
	RepositoryID   int64     `json:"repository_id"`
	Repository     string    `json:"repository"`
	EnvironmentID  uuid.UUID `json:"environment_id"`
	Environment    string    `json:"environment"`
	Reference      string    `json:"reference"`
	InstallationID int64     `json:"installation_id"`
}

func scopeForGrant(g grantState) runScope {
	return runScope{RepositoryID: g.Target.RepositoryID, Repository: g.Target.Owner + "/" + g.Target.Repository, EnvironmentID: g.EnvironmentID, Environment: g.Environment, Reference: g.Target.Reference, InstallationID: g.Connection.InstallationID}
}

func readScope(r *Run, raw []byte) (bool, error) {
	if len(raw) == 0 {
		// An older row has no historical snapshot. Do not invent it from a
		// current binding or admit provider work through that row.
		return false, nil
	}
	var scope runScope
	if json.Unmarshal(raw, &scope) != nil || scope.RepositoryID < 1 || scope.Repository == "" || scope.EnvironmentID == uuid.Nil || scope.Environment == "" || scope.Reference != r.Reference || scope.InstallationID < 1 {
		return false, ErrUnavailable
	}
	r.RepositoryID, r.Repository = scope.RepositoryID, scope.Repository
	r.EnvironmentID, r.Environment = scope.EnvironmentID, scope.Environment
	r.InstallationID = scope.InstallationID
	return true, nil
}

func scopeMatchesGrant(r runState, g grantState) bool {
	scope := scopeForGrant(g)
	return r.ScopeKnown && r.RepositoryID == scope.RepositoryID && r.Repository == scope.Repository && r.EnvironmentID == scope.EnvironmentID && r.Environment == scope.Environment && r.Reference == scope.Reference && r.InstallationID == scope.InstallationID
}
