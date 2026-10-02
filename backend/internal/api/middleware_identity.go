package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OperatorAdminChecker interface {
	IsPlatformAdmin(context.Context, uuid.UUID) (bool, error)
}

func RequireOperatorAdmin(checker OperatorAdminChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if humanSessionReference(c) == nil {
			WrapError(c, ErrForbidden)
			c.Abort()
			return
		}
		value, ok := c.Get("user_id")
		user, valid := value.(uuid.UUID)
		if !ok || !valid || user == uuid.Nil {
			WrapError(c, ErrUnauthorized)
			c.Abort()
			return
		}
		if checker == nil {
			WrapError(c, ErrForbidden)
			c.Abort()
			return
		}
		allowed, err := checker.IsPlatformAdmin(c.Request.Context(), user)
		if err != nil {
			WrapError(c, ErrServiceUnavailable)
			c.Abort()
			return
		}
		if !allowed {
			WrapError(c, ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}
