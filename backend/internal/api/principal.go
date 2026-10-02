package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/policy"
)

// PrincipalFromContext preserves workload identity separately from its human actor.
// Context values originate only in authenticated middleware, never request JSON.
func PrincipalFromContext(c *gin.Context) policy.Principal {
	actor, _ := c.Get("user_id")
	user, _ := actor.(uuid.UUID)
	p := policy.Principal{Kind: policy.Human, SubjectID: user, ActorID: user}
	if v, ok := c.Get("api_key_id"); ok {
		id, _ := v.(uuid.UUID)
		p.Kind = policy.APIKey
		p.SubjectID = id
		return p
	}
	if v, ok := c.Get("auth_claims"); ok {
		if claims, ok := v.(*auth.Claims); ok {
			if claims.ExpiresAt != nil {
				p.ExpiresAt = claims.ExpiresAt.Time
			}
			p.SessionID, _ = uuid.Parse(claims.SessionID)
			if claims.TokenType == "agent" {
				p.Kind = policy.AgentToken
				p.TokenID = claims.ID
				p.SubjectID, _ = uuid.Parse(claims.ID)
				if claims.LeaseID != nil {
					p.LeaseID = *claims.LeaseID
				}
			}
		}
	}
	return p
}
