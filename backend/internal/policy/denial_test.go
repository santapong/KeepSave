package policy

import (
	"reflect"
	"testing"
)

func TestPublicDenialVisibility(t *testing.T) {
	foreign := ProjectDenial(DeniedRole, false, true)
	missing := ProjectDenial(DeniedResource, false, true)
	if !reflect.DeepEqual(foreign, missing) {
		t.Fatal("foreign and missing resources distinguishable")
	}
	if ProjectDenial(DeniedRevoked, true, false).Code != "authentication_required" {
		t.Fatal("protected diagnostics allowed invalid session")
	}
	if ProjectDenial(DeniedScope, true, true).NextAction != "ask_workspace_admin" {
		t.Fatal("known resource lacks safe action")
	}
}
func TestTeamCapabilitiesDoNotInheritLegacyHierarchy(t *testing.T) {
	if CapabilityAllows("promoter", StartApprovedRun) || CapabilityAllows("viewer", ManageProfiles) || CapabilityAllows("editor", ManageConnections) {
		t.Fatal("unapproved capability inheritance")
	}
	if !CapabilityAllows("editor", StartApprovedRun) || !CapabilityAllows("promoter", ApproveProtected) {
		t.Fatal("explicit capabilities denied")
	}
}
