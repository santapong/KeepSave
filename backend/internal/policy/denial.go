package policy

// DenialReason is internal evidence. Transport adapters must project it after
// proving visibility; a reason never constitutes a new authority grant.
type DenialReason string

const (
	DeniedUnavailable DenialReason = "authority_unavailable"
	DeniedResource    DenialReason = "resource_unavailable"
	DeniedRole        DenialReason = "role_insufficient"
	DeniedScope       DenialReason = "scope_insufficient"
	DeniedExpired     DenialReason = "expired"
	DeniedRevoked     DenialReason = "revoked"
	DeniedRevision    DenialReason = "revision_changed"
)

type PublicDenial struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	NextAction string `json:"next_action,omitempty"`
}

func ProjectDenial(reason DenialReason, visible, authenticated bool) PublicDenial {
	if !authenticated {
		return PublicDenial{Code: "authentication_required", Message: "Sign in again to continue.", NextAction: "sign_in"}
	}
	generic := PublicDenial{Code: "resource_unavailable", Message: "This resource is unavailable."}
	if !visible {
		return generic
	}
	switch reason {
	case DeniedRole, DeniedScope:
		return PublicDenial{Code: "access_required", Message: "Your current access does not permit this operation.", NextAction: "ask_workspace_admin"}
	case DeniedExpired, DeniedRevoked:
		return PublicDenial{Code: "access_ended", Message: "This access has ended.", NextAction: "request_new_access"}
	case DeniedRevision:
		return PublicDenial{Code: "revision_changed", Message: "This resource changed. Refresh before trying again.", NextAction: "refresh"}
	default:
		return generic
	}
}
