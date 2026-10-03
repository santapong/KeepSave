package policy

// Team capabilities are explicit. This does not change the documented legacy
// role hierarchy or grant provider access without a binding/profile/run.
type Capability string

const (
	InspectTeam       Capability = "team.inspect"
	StartApprovedRun  Capability = "run.start"
	ManageConnections Capability = "connection.manage"
	ManageProfiles    Capability = "profile.manage"
	ApproveProtected  Capability = "protected.approve"
	ManageWorkloads   Capability = "workload.manage"
)

func CapabilityAllows(role string, c Capability) bool {
	if role == "admin" {
		return c == InspectTeam || c == StartApprovedRun || c == ManageConnections || c == ManageProfiles || c == ApproveProtected || c == ManageWorkloads
	}
	switch c {
	case InspectTeam:
		return role == "viewer" || role == "editor" || role == "promoter"
	case StartApprovedRun:
		return role == "editor"
	case ApproveProtected:
		return role == "promoter"
	default:
		return false
	}
}
