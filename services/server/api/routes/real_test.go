// SPDX-License-Identifier: Apache-2.0

package routes_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
)

// stubServer implements the enabled operations without behaviour; Build only registers them.
type stubServer struct{}

func (stubServer) ListGroups(context.Context, rest.ListGroupsRequestObject) (rest.ListGroupsResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateGroup(context.Context, rest.CreateGroupRequestObject) (rest.CreateGroupResponseObject, error) {
	return nil, nil
}

func (stubServer) GetGroup(context.Context, rest.GetGroupRequestObject) (rest.GetGroupResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateGroup(context.Context, rest.UpdateGroupRequestObject) (rest.UpdateGroupResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteGroup(context.Context, rest.DeleteGroupRequestObject) (rest.DeleteGroupResponseObject, error) {
	return nil, nil
}

func (stubServer) ListGroupMembers(context.Context, rest.ListGroupMembersRequestObject) (rest.ListGroupMembersResponseObject, error) {
	return nil, nil
}

func (stubServer) AddGroupMember(context.Context, rest.AddGroupMemberRequestObject) (rest.AddGroupMemberResponseObject, error) {
	return nil, nil
}

func (stubServer) RemoveGroupMember(context.Context, rest.RemoveGroupMemberRequestObject) (rest.RemoveGroupMemberResponseObject, error) {
	return nil, nil
}

func (stubServer) ListGrants(context.Context, rest.ListGrantsRequestObject) (rest.ListGrantsResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateGrant(context.Context, rest.CreateGrantRequestObject) (rest.CreateGrantResponseObject, error) {
	return nil, nil
}

func (stubServer) GetGrant(context.Context, rest.GetGrantRequestObject) (rest.GetGrantResponseObject, error) {
	return nil, nil
}

func (stubServer) RevokeGrant(context.Context, rest.RevokeGrantRequestObject) (rest.RevokeGrantResponseObject, error) {
	return nil, nil
}

func (stubServer) ListAuditEvents(context.Context, rest.ListAuditEventsRequestObject) (rest.ListAuditEventsResponseObject, error) {
	return nil, nil
}

func (stubServer) ExportAuditEvents(context.Context, rest.ExportAuditEventsRequestObject) (rest.ExportAuditEventsResponseObject, error) {
	return nil, nil
}

func (stubServer) VerifyAuditChain(context.Context, rest.VerifyAuditChainRequestObject) (rest.VerifyAuditChainResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateStepUpChallenge(context.Context, rest.CreateStepUpChallengeRequestObject) (rest.CreateStepUpChallengeResponseObject, error) {
	return nil, nil
}

func (stubServer) CompleteStepUpWithDeviceKey(context.Context, rest.CompleteStepUpWithDeviceKeyRequestObject) (rest.CompleteStepUpWithDeviceKeyResponseObject, error) {
	return nil, nil
}

func (stubServer) RefreshToken(context.Context, rest.RefreshTokenRequestObject) (rest.RefreshTokenResponseObject, error) {
	return nil, nil
}

func (stubServer) Logout(context.Context, rest.LogoutRequestObject) (rest.LogoutResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMySessions(context.Context, rest.ListMySessionsRequestObject) (rest.ListMySessionsResponseObject, error) {
	return nil, nil
}

func (stubServer) ListUserSessions(context.Context, rest.ListUserSessionsRequestObject) (rest.ListUserSessionsResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateExtensionSession(context.Context, rest.CreateExtensionSessionRequestObject) (rest.CreateExtensionSessionResponseObject, error) {
	return nil, nil
}

func (stubServer) GetDevice(context.Context, rest.GetDeviceRequestObject) (rest.GetDeviceResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMyDevices(context.Context, rest.ListMyDevicesRequestObject) (rest.ListMyDevicesResponseObject, error) {
	return nil, nil
}

func (stubServer) ListUserDevices(context.Context, rest.ListUserDevicesRequestObject) (rest.ListUserDevicesResponseObject, error) {
	return nil, nil
}

func (stubServer) AddOwner(context.Context, rest.AddOwnerRequestObject) (rest.AddOwnerResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateUser(context.Context, rest.CreateUserRequestObject) (rest.CreateUserResponseObject, error) {
	return nil, nil
}

func (stubServer) EnableUser(context.Context, rest.EnableUserRequestObject) (rest.EnableUserResponseObject, error) {
	return nil, nil
}

func (stubServer) GetMe(context.Context, rest.GetMeRequestObject) (rest.GetMeResponseObject, error) {
	return nil, nil
}

func (stubServer) GetUser(context.Context, rest.GetUserRequestObject) (rest.GetUserResponseObject, error) {
	return nil, nil
}

func (stubServer) ListOwners(context.Context, rest.ListOwnersRequestObject) (rest.ListOwnersResponseObject, error) {
	return nil, nil
}

func (stubServer) ListUsers(context.Context, rest.ListUsersRequestObject) (rest.ListUsersResponseObject, error) {
	return nil, nil
}

func (stubServer) RevokeOwner(context.Context, rest.RevokeOwnerRequestObject) (rest.RevokeOwnerResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateMyProfile(context.Context, rest.UpdateMyProfileRequestObject) (rest.UpdateMyProfileResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateUser(context.Context, rest.UpdateUserRequestObject) (rest.UpdateUserResponseObject, error) {
	return nil, nil
}

func (stubServer) GetJwks(context.Context, rest.GetJwksRequestObject) (rest.GetJwksResponseObject, error) {
	return nil, nil
}

func (stubServer) ListTokens(context.Context, rest.ListTokensRequestObject) (rest.ListTokensResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateToken(context.Context, rest.CreateTokenRequestObject) (rest.CreateTokenResponseObject, error) {
	return nil, nil
}

func (stubServer) RevokeToken(context.Context, rest.RevokeTokenRequestObject) (rest.RevokeTokenResponseObject, error) {
	return nil, nil
}

func (stubServer) RotateSigningKey(context.Context, rest.RotateSigningKeyRequestObject) (rest.RotateSigningKeyResponseObject, error) {
	return nil, nil
}

func (stubServer) GetDiscovery(context.Context, rest.GetDiscoveryRequestObject) (rest.GetDiscoveryResponseObject, error) {
	return nil, nil
}

func (stubServer) GetInstance(context.Context, rest.GetInstanceRequestObject) (rest.GetInstanceResponseObject, error) {
	return nil, nil
}

func (stubServer) ListFeatureFlags(context.Context, rest.ListFeatureFlagsRequestObject) (rest.ListFeatureFlagsResponseObject, error) {
	return nil, nil
}

func (stubServer) PutFeatureFlag(context.Context, rest.PutFeatureFlagRequestObject) (rest.PutFeatureFlagResponseObject, error) {
	return nil, nil
}

func (stubServer) ListInstanceSettings(context.Context, rest.ListInstanceSettingsRequestObject) (rest.ListInstanceSettingsResponseObject, error) {
	return nil, nil
}

func (stubServer) PutInstanceSetting(context.Context, rest.PutInstanceSettingRequestObject) (rest.PutInstanceSettingResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateBranding(context.Context, rest.UpdateBrandingRequestObject) (rest.UpdateBrandingResponseObject, error) {
	return nil, nil
}

func (stubServer) GetProblemType(context.Context, rest.GetProblemTypeRequestObject) (rest.GetProblemTypeResponseObject, error) {
	return nil, nil
}

func (stubServer) ListIdentityProviders(context.Context, rest.ListIdentityProvidersRequestObject) (rest.ListIdentityProvidersResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateIdentityProvider(context.Context, rest.CreateIdentityProviderRequestObject) (rest.CreateIdentityProviderResponseObject, error) {
	return nil, nil
}

func (stubServer) GetIdentityProvider(context.Context, rest.GetIdentityProviderRequestObject) (rest.GetIdentityProviderResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateIdentityProvider(context.Context, rest.UpdateIdentityProviderRequestObject) (rest.UpdateIdentityProviderResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteIdentityProvider(context.Context, rest.DeleteIdentityProviderRequestObject) (rest.DeleteIdentityProviderResponseObject, error) {
	return nil, nil
}

func (stubServer) GetMyCredentials(context.Context, rest.GetMyCredentialsRequestObject) (rest.GetMyCredentialsResponseObject, error) {
	return nil, nil
}

func (stubServer) GetUserCredentials(context.Context, rest.GetUserCredentialsRequestObject) (rest.GetUserCredentialsResponseObject, error) {
	return nil, nil
}

func (stubServer) UnlinkIdentity(context.Context, rest.UnlinkIdentityRequestObject) (rest.UnlinkIdentityResponseObject, error) {
	return nil, nil
}

func (stubServer) BeginPasskeyLogin(context.Context, rest.BeginPasskeyLoginRequestObject) (rest.BeginPasskeyLoginResponseObject, error) {
	return nil, nil
}

func (stubServer) CompletePasskeyLogin(context.Context, rest.CompletePasskeyLoginRequestObject) (rest.CompletePasskeyLoginResponseObject, error) {
	return nil, nil
}

func (stubServer) BeginEnrollment(context.Context, rest.BeginEnrollmentRequestObject) (rest.BeginEnrollmentResponseObject, error) {
	return nil, nil
}

func (stubServer) CompleteEnrollment(context.Context, rest.CompleteEnrollmentRequestObject) (rest.CompleteEnrollmentResponseObject, error) {
	return nil, nil
}

func (stubServer) BeginPasskeyRegistration(context.Context, rest.BeginPasskeyRegistrationRequestObject) (rest.BeginPasskeyRegistrationResponseObject, error) {
	return nil, nil
}

func (stubServer) CompletePasskeyRegistration(context.Context, rest.CompletePasskeyRegistrationRequestObject) (rest.CompletePasskeyRegistrationResponseObject, error) {
	return nil, nil
}

func (stubServer) RevokePasskey(context.Context, rest.RevokePasskeyRequestObject) (rest.RevokePasskeyResponseObject, error) {
	return nil, nil
}

func (stubServer) CompleteStepUpWithPasskey(context.Context, rest.CompleteStepUpWithPasskeyRequestObject) (rest.CompleteStepUpWithPasskeyResponseObject, error) {
	return nil, nil
}

func (stubServer) RedeemEnrollmentCode(context.Context, rest.RedeemEnrollmentCodeRequestObject) (rest.RedeemEnrollmentCodeResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateEnrollmentCode(context.Context, rest.CreateEnrollmentCodeRequestObject) (rest.CreateEnrollmentCodeResponseObject, error) {
	return nil, nil
}

func (stubServer) ApproveDevice(context.Context, rest.ApproveDeviceRequestObject) (rest.ApproveDeviceResponseObject, error) {
	return nil, nil
}

func (stubServer) BeginPairing(context.Context, rest.BeginPairingRequestObject) (rest.BeginPairingResponseObject, error) {
	return nil, nil
}

func (stubServer) ApprovePairing(context.Context, rest.ApprovePairingRequestObject) (rest.ApprovePairingResponseObject, error) {
	return nil, nil
}

func (stubServer) CompletePairing(context.Context, rest.CompletePairingRequestObject) (rest.CompletePairingResponseObject, error) {
	return nil, nil
}

func (stubServer) BeginOidcLogin(context.Context, rest.BeginOidcLoginRequestObject) (rest.BeginOidcLoginResponseObject, error) {
	return nil, nil
}

func (stubServer) HandleOidcCallback(context.Context, rest.HandleOidcCallbackRequestObject) (rest.HandleOidcCallbackResponseObject, error) {
	return nil, nil
}

func (stubServer) CompleteOidcLogin(context.Context, rest.CompleteOidcLoginRequestObject) (rest.CompleteOidcLoginResponseObject, error) {
	return nil, nil
}

func (stubServer) BeginIdentityLink(context.Context, rest.BeginIdentityLinkRequestObject) (rest.BeginIdentityLinkResponseObject, error) {
	return nil, nil
}

func (stubServer) CompleteIdentityLink(context.Context, rest.CompleteIdentityLinkRequestObject) (rest.CompleteIdentityLinkResponseObject, error) {
	return nil, nil
}

func (stubServer) LoginWithTrustedHeader(context.Context, rest.LoginWithTrustedHeaderRequestObject) (rest.LoginWithTrustedHeaderResponseObject, error) {
	return nil, nil
}

// TestRealTable_Enabled pins the enabled operations (P-04) with action and rate-limit class
// (AZ-P1): the three device reads of M1-33, the ten people, owner and self operations of M1-31
// and the JWKS and the rotation of the signing keys (M1-32), the sessions (M1-34), the step-up
// (M1-35), the providers and credentials (M1-36), the API tokens (M1-44), the audit log (M1-50),
// the enrollment codes (M1-37), the device approval and pairing (M1-38), the passkeys (M1-40),
// the OIDC login (M1-41) and the trusted header login (M1-43). A handler ticket that enables an
// operation changes this test together with
// api/operations.yaml.
func TestRealTable_Enabled(t *testing.T) {
	type entry struct{ pattern, action, class string }
	want := map[string]entry{
		"getDevice":        {"GET /api/v1/devices/{device_id}", "device.list", "rl.read"},
		"listMyDevices":    {"GET /api/v1/me/devices", "device.list", "rl.read"},
		"listUserDevices":  {"GET /api/v1/users/{user_id}/devices", "device.list", "rl.read"},
		"listUsers":        {"GET /api/v1/users", "user.read", "rl.read"},
		"createUser":       {"POST /api/v1/users", "user.create", "rl.admin"},
		"getUser":          {"GET /api/v1/users/{user_id}", "user.read", "rl.read"},
		"updateUser":       {"PATCH /api/v1/users/{user_id}", "user.update", "rl.admin"},
		"enableUser":       {"POST /api/v1/users/{user_id}/enable", "user.enable", "rl.admin"},
		"listOwners":       {"GET /api/v1/owners", "user.read", "rl.read"},
		"addOwner":         {"POST /api/v1/owners", "owner.add", "rl.admin"},
		"revokeOwner":      {"POST /api/v1/owners/{user_id}/revoke", "owner.remove", "rl.admin"},
		"getMe":            {"GET /api/v1/me", "self.read", "rl.read"},
		"updateMyProfile":  {"PATCH /api/v1/me/profile", "self.profile.update", "rl.write"},
		"getJwks":          {"GET /api/v1/instance/jwks", "instance.discovery.read", "rl.public_auth"},
		"rotateSigningKey": {"POST /api/v1/signing-keys/rotate", "signing_key.rotate", "rl.admin"},
		// M1-34
		"refreshToken":           {"POST /api/v1/auth/token/refresh", "auth.token.refresh", "rl.public_auth"},
		"logout":                 {"POST /api/v1/auth/logout", "auth.logout", "rl.write"},
		"listMySessions":         {"GET /api/v1/me/sessions", "session.list", "rl.read"},
		"listUserSessions":       {"GET /api/v1/users/{user_id}/sessions", "session.list", "rl.read"},
		"createExtensionSession": {"POST /api/v1/me/sessions/extensions", "session.extension.create", "rl.write"},
		// M1-35
		"createStepUpChallenge":       {"POST /api/v1/auth/step-up/challenge", "auth.step_up", "rl.public_auth"},
		"completeStepUpWithDeviceKey": {"POST /api/v1/auth/step-up/device-key", "auth.step_up", "rl.public_auth"},
		// M1-36
		"listIdentityProviders":  {"GET /api/v1/identity-providers", "identity_provider.read", "rl.read"},
		"createIdentityProvider": {"POST /api/v1/identity-providers", "identity_provider.manage", "rl.admin"},
		"getIdentityProvider":    {"GET /api/v1/identity-providers/{provider_id}", "identity_provider.read", "rl.read"},
		"updateIdentityProvider": {"PATCH /api/v1/identity-providers/{provider_id}", "identity_provider.manage", "rl.admin"},
		"deleteIdentityProvider": {"DELETE /api/v1/identity-providers/{provider_id}", "identity_provider.manage", "rl.admin"},
		"getMyCredentials":       {"GET /api/v1/me/credentials", "identity.read", "rl.read"},
		"getUserCredentials":     {"GET /api/v1/users/{user_id}/credentials", "identity.read", "rl.read"},
		"unlinkIdentity":         {"DELETE /api/v1/users/{user_id}/identities/{identity_id}", "identity.unlink", "rl.admin"},
		// M1-44
		"listTokens":  {"GET /api/v1/principals/{principal_id}/tokens", "token.list", "rl.read"},
		"createToken": {"POST /api/v1/principals/{principal_id}/tokens", "token.create", "rl.admin"},
		"revokeToken": {"DELETE /api/v1/tokens/{token_id}", "token.revoke", "rl.admin"},
		// M1-48
		"listGroups":        {"GET /api/v1/groups", "group.read", "rl.read"},
		"createGroup":       {"POST /api/v1/groups", "group.create", "rl.admin"},
		"getGroup":          {"GET /api/v1/groups/{group_id}", "group.read", "rl.read"},
		"updateGroup":       {"PATCH /api/v1/groups/{group_id}", "group.update", "rl.admin"},
		"deleteGroup":       {"DELETE /api/v1/groups/{group_id}", "group.delete", "rl.admin"},
		"listGroupMembers":  {"GET /api/v1/groups/{group_id}/members", "group.read", "rl.read"},
		"addGroupMember":    {"POST /api/v1/groups/{group_id}/members", "group.member.add", "rl.admin"},
		"removeGroupMember": {"DELETE /api/v1/groups/{group_id}/members/{user_id}", "group.member.remove", "rl.admin"},
		"listGrants":        {"GET /api/v1/grants", "grant.read", "rl.read"},
		"createGrant":       {"POST /api/v1/grants", "grant.create", "rl.admin"},
		"getGrant":          {"GET /api/v1/grants/{grant_id}", "grant.read", "rl.read"},
		"revokeGrant":       {"POST /api/v1/grants/{grant_id}/revoke", "grant.revoke", "rl.admin"},
		// M1-50
		"listAuditEvents":   {"GET /api/v1/audit/events", "audit.read", "rl.admin"},
		"exportAuditEvents": {"GET /api/v1/audit/export", "audit.read", "rl.admin"},
		"verifyAuditChain":  {"POST /api/v1/audit/verify", "audit.verify", "rl.admin"},
		// M1-37
		"redeemEnrollmentCode": {"POST /api/v1/auth/enroll/redeem", "auth.login.complete", "rl.public_auth"},
		"createEnrollmentCode": {"POST /api/v1/users/{user_id}/enrollment-codes", "user.update", "rl.admin"},
		// M1-38
		"approveDevice":   {"POST /api/v1/devices/{device_id}/approve", "device.approve", "rl.admin"},
		"approvePairing":  {"POST /api/v1/me/devices/pairings", "device.approve", "rl.admin"},
		"beginPairing":    {"POST /api/v1/auth/pairing/begin", "auth.login.begin", "rl.public_auth"},
		"completePairing": {"POST /api/v1/auth/pairing/complete", "auth.login.complete", "rl.public_auth"},
		// M1-46
		"getDiscovery":         {"GET /api/v1/instance/discovery", "instance.discovery.read", "rl.public_auth"},
		"getInstance":          {"GET /api/v1/instance", "instance.read", "rl.read"},
		"listInstanceSettings": {"GET /api/v1/instance/settings", "instance.settings.read", "rl.read"},
		"putInstanceSetting":   {"PUT /api/v1/instance/settings/{key}", "instance.settings.update", "rl.admin"},
		"updateBranding":       {"PUT /api/v1/instance/branding", "instance.branding.update", "rl.admin"},
		"getProblemType":       {"GET /api/v1/problems/{code}", "instance.discovery.read", "rl.public_auth"},
		// M1-40
		"beginPasskeyLogin":           {"POST /api/v1/auth/passkey/begin", "auth.login.begin", "rl.public_auth"},
		"completePasskeyLogin":        {"POST /api/v1/auth/passkey/complete", "auth.login.complete", "rl.public_auth"},
		"beginEnrollment":             {"POST /api/v1/auth/enroll/begin", "auth.login.begin", "rl.public_auth"},
		"completeEnrollment":          {"POST /api/v1/auth/enroll/complete", "auth.login.complete", "rl.public_auth"},
		"beginPasskeyRegistration":    {"POST /api/v1/me/passkeys/begin", "passkey.register", "rl.admin"},
		"completePasskeyRegistration": {"POST /api/v1/me/passkeys/complete", "passkey.register", "rl.admin"},
		"revokePasskey":               {"DELETE /api/v1/users/{user_id}/passkeys/{passkey_id}", "passkey.revoke", "rl.admin"},
		"completeStepUpWithPasskey":   {"POST /api/v1/auth/step-up/passkey", "auth.step_up", "rl.public_auth"},
		// M1-41
		"beginOidcLogin":       {"POST /api/v1/auth/oidc/begin", "auth.login.begin", "rl.public_auth"},
		"handleOidcCallback":   {"GET /api/v1/auth/oidc/callback", "auth.login.begin", "rl.public_auth"},
		"completeOidcLogin":    {"POST /api/v1/auth/oidc/complete", "auth.login.complete", "rl.public_auth"},
		"beginIdentityLink":    {"POST /api/v1/me/identities/begin", "identity.link", "rl.admin"},
		"completeIdentityLink": {"POST /api/v1/me/identities/complete", "identity.link", "rl.admin"},
		// M1-43
		"loginWithTrustedHeader": {"POST /api/v1/auth/trusted-header/login", "auth.login.complete", "rl.public_auth"},
		// M1-47
		"listFeatureFlags": {"GET /api/v1/feature-flags", "feature_flag.read", "rl.read"},
		"putFeatureFlag":   {"PUT /api/v1/feature-flags/{key}", "feature_flag.update", "rl.admin"},
	}
	ops := routes.Enabled()
	if len(ops) != len(want) {
		t.Fatalf("Enabled() = %d operations, want %d", len(ops), len(want))
	}
	for _, op := range ops {
		if w := want[op.ID]; w.pattern != op.Pattern || op.Action != w.action || op.RateLimitClass != w.class {
			t.Errorf("operation %+v, want %+v", op, w)
		}
		// Only the export streams a body that is not JSON (route class of downloads, M1-50).
		if op.Download != (op.ID == "exportAuditEvents") {
			t.Errorf("%s: Download = %v", op.ID, op.Download)
		}
		if got, ok := routes.ByPattern(op.Pattern); !ok || got.ID != op.ID {
			t.Errorf("ByPattern(%s) = %+v, %v", op.Pattern, got, ok)
		}
	}
	got, err := routes.Build(stubServer{}, routes.Options{}, routes.SpecCatalog())
	if err != nil || len(got) != len(want) {
		t.Fatalf("Build = %d routes, %v", len(got), err)
	}
	if err := routes.Verify(nil, routes.SpecCatalog()); err == nil {
		t.Error("Verify(nothing) passed although operations are enabled")
	}
	if err := routes.Verify([]string{"GET /api/v1/discovery"}, routes.SpecCatalog()); err == nil {
		t.Error("Verify accepted a pattern of no enabled operation")
	}
	if _, ok := routes.ByID("abortUpload"); ok {
		t.Error("ByID found an operation that is not enabled")
	}
	if _, ok := routes.ByPattern("GET /api/v1/discovery"); ok {
		t.Error("ByPattern found an operation that is not enabled")
	}
}

// TestRealTable_PathsBelowBaseURL holds W6 for every enabled operation.
func TestRealTable_PathsBelowBaseURL(t *testing.T) {
	for _, op := range routes.Enabled() {
		if !strings.HasPrefix(op.Path, routes.BaseURL+"/") || !strings.Contains(op.Pattern, " "+routes.BaseURL+"/") {
			t.Errorf("%s: path %q, pattern %q not below %s", op.ID, op.Path, op.Pattern, routes.BaseURL)
		}
	}
}

// TestSpecCatalog_MatchesActionsYAML: the generated catalogue knows every action of
// docs/spec/actions.yaml that a route may carry, with its class, and no system action and no
// action without a class.
func TestSpecCatalog_MatchesActionsYAML(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/spec/actions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Actions []struct {
			ID      string `yaml:"id"`
			Right   string `yaml:"right"`
			RLClass string `yaml:"rl_class"`
		} `yaml:"actions"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Actions) == 0 {
		t.Fatal("no actions read")
	}
	cat := routes.SpecCatalog()
	for _, a := range file.Actions {
		class, ok := cat.Class(a.ID)
		unreachable := a.Right == "system" || a.RLClass == "none"
		switch {
		case unreachable && ok:
			t.Errorf("action %s (right %q, class %q) is known to the route catalogue", a.ID, a.Right, a.RLClass)
		case !unreachable && (!ok || class != a.RLClass):
			t.Errorf("action %s: catalogue gives %q (known %v), actions.yaml %q", a.ID, class, ok, a.RLClass)
		}
	}
}
