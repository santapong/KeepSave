package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type FeedbackHandler struct {
	feedbackService *service.FeedbackService
}

func NewFeedbackHandler(feedbackService *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{feedbackService: feedbackService}
}

type SubmitFeedbackRequest struct {
	Category string `json:"category" binding:"required,oneof=bug idea other"`
	Message  string `json:"message" binding:"required,max=4000"`
	PageURL  string `json:"page_url" binding:"omitempty,max=500"`
}

// errFeedbackUpstream is the sentinel for GitHub-side failures. The client
// only ever sees the generic message; the underlying cause (which may embed
// the GitHub response body) is logged server-side by WrapError.
var errFeedbackUpstream = &HTTPError{Symbol: "UPSTREAM_ERROR", Status: http.StatusBadGateway, Message: "failed to file feedback"}

func (h *FeedbackHandler) Submit(c *gin.Context) {
	if !h.feedbackService.Enabled() {
		RespondError(c, http.StatusServiceUnavailable, "feedback is not configured on this server")
		return
	}

	var req SubmitFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WrapError(c, Wrap(ErrInvalidInput, err))
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		RespondError(c, http.StatusBadRequest, "message must not be empty")
		return
	}

	// Identity + User-Agent come from the auth context / request headers,
	// never from client JSON.
	userID, authedOK := getUserID(c)
	if !authedOK {
		return
	}
	emailVal, _ := c.Get("email")
	email, _ := emailVal.(string)

	issueURL, issueNumber, err := h.feedbackService.Submit(
		userID, email, req.Category, message, req.PageURL,
		c.Request.UserAgent(), c.ClientIP(),
	)
	if err != nil {
		WrapError(c, Wrap(errFeedbackUpstream, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"issue_url": issueURL, "issue_number": issueNumber})
}
