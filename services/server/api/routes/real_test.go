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

func (stubServer) ExplainSpaceAccess(context.Context, rest.ExplainSpaceAccessRequestObject) (rest.ExplainSpaceAccessResponseObject, error) {
	return nil, nil
}

func (stubServer) ExplainChannelAccess(context.Context, rest.ExplainChannelAccessRequestObject) (rest.ExplainChannelAccessResponseObject, error) {
	return nil, nil
}

func (stubServer) PreviewGrant(context.Context, rest.PreviewGrantRequestObject) (rest.PreviewGrantResponseObject, error) {
	return nil, nil
}

func (stubServer) ViewAsUser(context.Context, rest.ViewAsUserRequestObject) (rest.ViewAsUserResponseObject, error) {
	return nil, nil
}

func (stubServer) ListApps(context.Context, rest.ListAppsRequestObject) (rest.ListAppsResponseObject, error) {
	return nil, nil
}

func (stubServer) InstallApp(context.Context, rest.InstallAppRequestObject) (rest.InstallAppResponseObject, error) {
	return nil, nil
}

func (stubServer) ValidateManifest(context.Context, rest.ValidateManifestRequestObject) (rest.ValidateManifestResponseObject, error) {
	return nil, nil
}

func (stubServer) GetApp(context.Context, rest.GetAppRequestObject) (rest.GetAppResponseObject, error) {
	return nil, nil
}

func (stubServer) UpgradeAppManifest(context.Context, rest.UpgradeAppManifestRequestObject) (rest.UpgradeAppManifestResponseObject, error) {
	return nil, nil
}

func (stubServer) SetAppScopes(context.Context, rest.SetAppScopesRequestObject) (rest.SetAppScopesResponseObject, error) {
	return nil, nil
}

func (stubServer) SuspendApp(context.Context, rest.SuspendAppRequestObject) (rest.SuspendAppResponseObject, error) {
	return nil, nil
}

func (stubServer) ResumeApp(context.Context, rest.ResumeAppRequestObject) (rest.ResumeAppResponseObject, error) {
	return nil, nil
}

func (stubServer) UninstallApp(context.Context, rest.UninstallAppRequestObject) (rest.UninstallAppResponseObject, error) {
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

func (stubServer) ListWebhookSubscriptions(context.Context, rest.ListWebhookSubscriptionsRequestObject) (rest.ListWebhookSubscriptionsResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateWebhookSubscription(context.Context, rest.CreateWebhookSubscriptionRequestObject) (rest.CreateWebhookSubscriptionResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateWebhookSubscription(context.Context, rest.UpdateWebhookSubscriptionRequestObject) (rest.UpdateWebhookSubscriptionResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteWebhookSubscription(context.Context, rest.DeleteWebhookSubscriptionRequestObject) (rest.DeleteWebhookSubscriptionResponseObject, error) {
	return nil, nil
}

func (stubServer) ListWebhookDeliveries(context.Context, rest.ListWebhookDeliveriesRequestObject) (rest.ListWebhookDeliveriesResponseObject, error) {
	return nil, nil
}

func (stubServer) RedeliverWebhook(context.Context, rest.RedeliverWebhookRequestObject) (rest.RedeliverWebhookResponseObject, error) {
	return nil, nil
}

func (stubServer) GetWebhookKeys(context.Context, rest.GetWebhookKeysRequestObject) (rest.GetWebhookKeysResponseObject, error) {
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

func (stubServer) DisableUser(context.Context, rest.DisableUserRequestObject) (rest.DisableUserResponseObject, error) {
	return nil, nil
}

func (stubServer) RevokeDevice(context.Context, rest.RevokeDeviceRequestObject) (rest.RevokeDeviceResponseObject, error) {
	return nil, nil
}

func (stubServer) RevokeSession(context.Context, rest.RevokeSessionRequestObject) (rest.RevokeSessionResponseObject, error) {
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

func (stubServer) ListSpaces(context.Context, rest.ListSpacesRequestObject) (rest.ListSpacesResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateSpace(context.Context, rest.CreateSpaceRequestObject) (rest.CreateSpaceResponseObject, error) {
	return nil, nil
}

func (stubServer) GetSpace(context.Context, rest.GetSpaceRequestObject) (rest.GetSpaceResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateSpace(context.Context, rest.UpdateSpaceRequestObject) (rest.UpdateSpaceResponseObject, error) {
	return nil, nil
}

func (stubServer) MoveSpace(context.Context, rest.MoveSpaceRequestObject) (rest.MoveSpaceResponseObject, error) {
	return nil, nil
}

func (stubServer) ArchiveSpace(context.Context, rest.ArchiveSpaceRequestObject) (rest.ArchiveSpaceResponseObject, error) {
	return nil, nil
}

func (stubServer) PutMySidebarSpace(context.Context, rest.PutMySidebarSpaceRequestObject) (rest.PutMySidebarSpaceResponseObject, error) {
	return nil, nil
}

func (stubServer) ListHosts(context.Context, rest.ListHostsRequestObject) (rest.ListHostsResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateHost(context.Context, rest.CreateHostRequestObject) (rest.CreateHostResponseObject, error) {
	return nil, nil
}

func (stubServer) GetHost(context.Context, rest.GetHostRequestObject) (rest.GetHostResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateHost(context.Context, rest.UpdateHostRequestObject) (rest.UpdateHostResponseObject, error) {
	return nil, nil
}

func (stubServer) SetHostState(context.Context, rest.SetHostStateRequestObject) (rest.SetHostStateResponseObject, error) {
	return nil, nil
}

func (stubServer) ListSpaceChannels(context.Context, rest.ListSpaceChannelsRequestObject) (rest.ListSpaceChannelsResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateChannel(context.Context, rest.CreateChannelRequestObject) (rest.CreateChannelResponseObject, error) {
	return nil, nil
}

func (stubServer) GetChannel(context.Context, rest.GetChannelRequestObject) (rest.GetChannelResponseObject, error) {
	return nil, nil
}

func (stubServer) GetChannelBehavior(context.Context, rest.GetChannelBehaviorRequestObject) (rest.GetChannelBehaviorResponseObject, error) {
	return nil, nil
}

func (stubServer) PutChannelBehavior(context.Context, rest.PutChannelBehaviorRequestObject) (rest.PutChannelBehaviorResponseObject, error) {
	return nil, nil
}

func (stubServer) ListServedChannels(context.Context, rest.ListServedChannelsRequestObject) (rest.ListServedChannelsResponseObject, error) {
	return nil, nil
}

func (stubServer) GetServedChannelConfig(context.Context, rest.GetServedChannelConfigRequestObject) (rest.GetServedChannelConfigResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateChannel(context.Context, rest.UpdateChannelRequestObject) (rest.UpdateChannelResponseObject, error) {
	return nil, nil
}

func (stubServer) MoveChannel(context.Context, rest.MoveChannelRequestObject) (rest.MoveChannelResponseObject, error) {
	return nil, nil
}

func (stubServer) ArchiveChannel(context.Context, rest.ArchiveChannelRequestObject) (rest.ArchiveChannelResponseObject, error) {
	return nil, nil
}

func (stubServer) SetSpaceOperator(context.Context, rest.SetSpaceOperatorRequestObject) (rest.SetSpaceOperatorResponseObject, error) {
	return nil, nil
}

func (stubServer) SetChannelOperator(context.Context, rest.SetChannelOperatorRequestObject) (rest.SetChannelOperatorResponseObject, error) {
	return nil, nil
}

func (stubServer) ListForumTags(context.Context, rest.ListForumTagsRequestObject) (rest.ListForumTagsResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateForumTag(context.Context, rest.CreateForumTagRequestObject) (rest.CreateForumTagResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateForumTag(context.Context, rest.UpdateForumTagRequestObject) (rest.UpdateForumTagResponseObject, error) {
	return nil, nil
}

func (stubServer) MoveForumTag(context.Context, rest.MoveForumTagRequestObject) (rest.MoveForumTagResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteForumTag(context.Context, rest.DeleteForumTagRequestObject) (rest.DeleteForumTagResponseObject, error) {
	return nil, nil
}

func (stubServer) ListChannelThreads(context.Context, rest.ListChannelThreadsRequestObject) (rest.ListChannelThreadsResponseObject, error) {
	return nil, nil
}

func (stubServer) GetThread(context.Context, rest.GetThreadRequestObject) (rest.GetThreadResponseObject, error) {
	return nil, nil
}

func (stubServer) SetThreadTitle(context.Context, rest.SetThreadTitleRequestObject) (rest.SetThreadTitleResponseObject, error) {
	return nil, nil
}

func (stubServer) CloseThread(context.Context, rest.CloseThreadRequestObject) (rest.CloseThreadResponseObject, error) {
	return nil, nil
}

func (stubServer) ReopenThread(context.Context, rest.ReopenThreadRequestObject) (rest.ReopenThreadResponseObject, error) {
	return nil, nil
}

func (stubServer) MoveThread(context.Context, rest.MoveThreadRequestObject) (rest.MoveThreadResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateMessage(context.Context, rest.CreateMessageRequestObject) (rest.CreateMessageResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateThread(context.Context, rest.CreateThreadRequestObject) (rest.CreateThreadResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMyMentions(context.Context, rest.ListMyMentionsRequestObject) (rest.ListMyMentionsResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMyTemplates(context.Context, rest.ListMyTemplatesRequestObject) (rest.ListMyTemplatesResponseObject, error) {
	return nil, nil
}

func (stubServer) ListTemplates(context.Context, rest.ListTemplatesRequestObject) (rest.ListTemplatesResponseObject, error) {
	return nil, nil
}

func (stubServer) CreatePersonalTemplate(context.Context, rest.CreatePersonalTemplateRequestObject) (rest.CreatePersonalTemplateResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateChannelTemplate(context.Context, rest.CreateChannelTemplateRequestObject) (rest.CreateChannelTemplateResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateTemplate(context.Context, rest.UpdateTemplateRequestObject) (rest.UpdateTemplateResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteTemplate(context.Context, rest.DeleteTemplateRequestObject) (rest.DeleteTemplateResponseObject, error) {
	return nil, nil
}

func (stubServer) ListThreadMessages(context.Context, rest.ListThreadMessagesRequestObject) (rest.ListThreadMessagesResponseObject, error) {
	return nil, nil
}

func (stubServer) GetMessage(context.Context, rest.GetMessageRequestObject) (rest.GetMessageResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateMessage(context.Context, rest.UpdateMessageRequestObject) (rest.UpdateMessageResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteMessage(context.Context, rest.DeleteMessageRequestObject) (rest.DeleteMessageResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMessageRevisions(context.Context, rest.ListMessageRevisionsRequestObject) (rest.ListMessageRevisionsResponseObject, error) {
	return nil, nil
}

func (stubServer) AddReaction(context.Context, rest.AddReactionRequestObject) (rest.AddReactionResponseObject, error) {
	return nil, nil
}

func (stubServer) RemoveReaction(context.Context, rest.RemoveReactionRequestObject) (rest.RemoveReactionResponseObject, error) {
	return nil, nil
}

func (stubServer) ListReactions(context.Context, rest.ListReactionsRequestObject) (rest.ListReactionsResponseObject, error) {
	return nil, nil
}

func (stubServer) PinMessage(context.Context, rest.PinMessageRequestObject) (rest.PinMessageResponseObject, error) {
	return nil, nil
}

func (stubServer) UnpinMessage(context.Context, rest.UnpinMessageRequestObject) (rest.UnpinMessageResponseObject, error) {
	return nil, nil
}

func (stubServer) ListPins(context.Context, rest.ListPinsRequestObject) (rest.ListPinsResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMyReadStates(context.Context, rest.ListMyReadStatesRequestObject) (rest.ListMyReadStatesResponseObject, error) {
	return nil, nil
}

func (stubServer) PutReadState(context.Context, rest.PutReadStateRequestObject) (rest.PutReadStateResponseObject, error) {
	return nil, nil
}

func (stubServer) MarkThreadUnread(context.Context, rest.MarkThreadUnreadRequestObject) (rest.MarkThreadUnreadResponseObject, error) {
	return nil, nil
}

func (stubServer) PutThreadDraft(context.Context, rest.PutThreadDraftRequestObject) (rest.PutThreadDraftResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteThreadDraft(context.Context, rest.DeleteThreadDraftRequestObject) (rest.DeleteThreadDraftResponseObject, error) {
	return nil, nil
}

func (stubServer) PutChannelDraft(context.Context, rest.PutChannelDraftRequestObject) (rest.PutChannelDraftResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteChannelDraft(context.Context, rest.DeleteChannelDraftRequestObject) (rest.DeleteChannelDraftResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMyDrafts(context.Context, rest.ListMyDraftsRequestObject) (rest.ListMyDraftsResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMySettings(context.Context, rest.ListMySettingsRequestObject) (rest.ListMySettingsResponseObject, error) {
	return nil, nil
}

func (stubServer) PutMySetting(context.Context, rest.PutMySettingRequestObject) (rest.PutMySettingResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteMySetting(context.Context, rest.DeleteMySettingRequestObject) (rest.DeleteMySettingResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMyNotificationPreferences(context.Context, rest.ListMyNotificationPreferencesRequestObject) (rest.ListMyNotificationPreferencesResponseObject, error) {
	return nil, nil
}

func (stubServer) PutNotificationPreference(context.Context, rest.PutNotificationPreferenceRequestObject) (rest.PutNotificationPreferenceResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteNotificationPreference(context.Context, rest.DeleteNotificationPreferenceRequestObject) (rest.DeleteNotificationPreferenceResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMyQuietHours(context.Context, rest.ListMyQuietHoursRequestObject) (rest.ListMyQuietHoursResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateQuietHoursRule(context.Context, rest.CreateQuietHoursRuleRequestObject) (rest.CreateQuietHoursRuleResponseObject, error) {
	return nil, nil
}

func (stubServer) PutQuietHoursRule(context.Context, rest.PutQuietHoursRuleRequestObject) (rest.PutQuietHoursRuleResponseObject, error) {
	return nil, nil
}

func (stubServer) DeleteQuietHoursRule(context.Context, rest.DeleteQuietHoursRuleRequestObject) (rest.DeleteQuietHoursRuleResponseObject, error) {
	return nil, nil
}

func (stubServer) GetMySidebar(context.Context, rest.GetMySidebarRequestObject) (rest.GetMySidebarResponseObject, error) {
	return nil, nil
}

func (stubServer) ListEvents(context.Context, rest.ListEventsRequestObject) (rest.ListEventsResponseObject, error) {
	return nil, nil
}

func (stubServer) GetSnapshot(context.Context, rest.GetSnapshotRequestObject) (rest.GetSnapshotResponseObject, error) {
	return nil, nil
}

func (stubServer) SetThreadTags(context.Context, rest.SetThreadTagsRequestObject) (rest.SetThreadTagsResponseObject, error) {
	return nil, nil
}

func (stubServer) SetHostLlmCredentials(context.Context, rest.SetHostLlmCredentialsRequestObject) (rest.SetHostLlmCredentialsResponseObject, error) {
	return nil, nil
}

func (stubServer) IssueHostPairingCode(context.Context, rest.IssueHostPairingCodeRequestObject) (rest.IssueHostPairingCodeResponseObject, error) {
	return nil, nil
}

func (stubServer) PairHost(context.Context, rest.PairHostRequestObject) (rest.PairHostResponseObject, error) {
	return nil, nil
}

func (stubServer) ConfirmHost(context.Context, rest.ConfirmHostRequestObject) (rest.ConfirmHostResponseObject, error) {
	return nil, nil
}

func (stubServer) ListAgents(context.Context, rest.ListAgentsRequestObject) (rest.ListAgentsResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateAgent(context.Context, rest.CreateAgentRequestObject) (rest.CreateAgentResponseObject, error) {
	return nil, nil
}

func (stubServer) GetAgent(context.Context, rest.GetAgentRequestObject) (rest.GetAgentResponseObject, error) {
	return nil, nil
}

func (stubServer) GetConsumerState(context.Context, rest.GetConsumerStateRequestObject) (rest.GetConsumerStateResponseObject, error) {
	return nil, nil
}

func (stubServer) PollAgentEvents(context.Context, rest.PollAgentEventsRequestObject) (rest.PollAgentEventsResponseObject, error) {
	return nil, nil
}

func (stubServer) AckAgentEvents(context.Context, rest.AckAgentEventsRequestObject) (rest.AckAgentEventsResponseObject, error) {
	return nil, nil
}

func (stubServer) UpdateAgent(context.Context, rest.UpdateAgentRequestObject) (rest.UpdateAgentResponseObject, error) {
	return nil, nil
}

func (stubServer) DisableAgent(context.Context, rest.DisableAgentRequestObject) (rest.DisableAgentResponseObject, error) {
	return nil, nil
}

func (stubServer) EnableAgent(context.Context, rest.EnableAgentRequestObject) (rest.EnableAgentResponseObject, error) {
	return nil, nil
}

func (stubServer) SetAgentBillingMode(context.Context, rest.SetAgentBillingModeRequestObject) (rest.SetAgentBillingModeResponseObject, error) {
	return nil, nil
}

func (stubServer) SetAllAgentsBillingMode(context.Context, rest.SetAllAgentsBillingModeRequestObject) (rest.SetAllAgentsBillingModeResponseObject, error) {
	return nil, nil
}

func (stubServer) SetCredentialHolder(context.Context, rest.SetCredentialHolderRequestObject) (rest.SetCredentialHolderResponseObject, error) {
	return nil, nil
}

func (stubServer) SetAgentToolClass(context.Context, rest.SetAgentToolClassRequestObject) (rest.SetAgentToolClassResponseObject, error) {
	return nil, nil
}

func (stubServer) GetConsoleSession(context.Context, rest.GetConsoleSessionRequestObject) (rest.GetConsoleSessionResponseObject, error) {
	return nil, nil
}

func (stubServer) EndViewAs(context.Context, rest.EndViewAsRequestObject) (rest.EndViewAsResponseObject, error) {
	return nil, nil
}

func (stubServer) StepUpWithPassword(context.Context, rest.StepUpWithPasswordRequestObject) (rest.StepUpWithPasswordResponseObject, error) {
	return nil, nil
}

func (stubServer) BeginStepUpOidcReauth(context.Context, rest.BeginStepUpOidcReauthRequestObject) (rest.BeginStepUpOidcReauthResponseObject, error) {
	return nil, nil
}

func (stubServer) CompleteStepUpOidcReauth(context.Context, rest.CompleteStepUpOidcReauthRequestObject) (rest.CompleteStepUpOidcReauthResponseObject, error) {
	return nil, nil
}

func (stubServer) LoginWithPassword(context.Context, rest.LoginWithPasswordRequestObject) (rest.LoginWithPasswordResponseObject, error) {
	return nil, nil
}

func (stubServer) SetMyLocalCredential(context.Context, rest.SetMyLocalCredentialRequestObject) (rest.SetMyLocalCredentialResponseObject, error) {
	return nil, nil
}

func (stubServer) CreateUpload(context.Context, rest.CreateUploadRequestObject) (rest.CreateUploadResponseObject, error) {
	return nil, nil
}

func (stubServer) UploadPart(context.Context, rest.UploadPartRequestObject) (rest.UploadPartResponseObject, error) {
	return nil, nil
}

func (stubServer) GetUpload(context.Context, rest.GetUploadRequestObject) (rest.GetUploadResponseObject, error) {
	return nil, nil
}

func (stubServer) CompleteUpload(context.Context, rest.CompleteUploadRequestObject) (rest.CompleteUploadResponseObject, error) {
	return nil, nil
}

func (stubServer) AbortUpload(context.Context, rest.AbortUploadRequestObject) (rest.AbortUploadResponseObject, error) {
	return nil, nil
}

func (stubServer) GetFile(context.Context, rest.GetFileRequestObject) (rest.GetFileResponseObject, error) {
	return nil, nil
}

// TestRealTable_Enabled pins the enabled operations (P-04) with action and rate-limit class
// (AZ-P1): the three device reads of M1-33, the ten people, owner and self operations of M1-31
// and the JWKS and the rotation of the signing keys (M1-32), the sessions (M1-34), the step-up
// (M1-35), the providers and credentials (M1-36), the API tokens (M1-44), the audit log (M1-50),
// the enrollment codes (M1-37), the device approval and pairing (M1-38), the passkeys (M1-40),
// the OIDC login (M1-41), the trusted header login (M1-43), the revocations (M1-45), the host
// records (M2-39), the local accounts (M1-39), the nine app operations (M2-24), the threads
// (M1-55), sending (M1-56), editing and deleting (M1-58), the preferences (M3-18) and the upload
// sessions with getFile (M3-03). A handler
// ticket that enables an operation changes this test together with
// api/operations.yaml.
func TestRealTable_Enabled(t *testing.T) {
	type entry struct{ pattern, action, class string }
	want := map[string]entry{
		"getDevice":        {"GET /api/v1/devices/{device_id}", "device.list", "rl.read"},
		"createUpload":     {"POST /api/v1/channels/{channel_id}/uploads", "file.upload", "rl.upload"},
		"uploadPart":       {"PUT /api/v1/uploads/{file_id}/parts/{part_number}", "file.upload", "rl.upload"},
		"getUpload":        {"GET /api/v1/uploads/{file_id}", "file.upload", "rl.upload"},
		"completeUpload":   {"POST /api/v1/uploads/{file_id}/complete", "file.upload", "rl.upload"},
		"abortUpload":      {"DELETE /api/v1/uploads/{file_id}", "file.delete", "rl.write"},
		"getFile":          {"GET /api/v1/files/{file_id}", "file.read", "rl.read"},
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
		// M1-49
		"explainSpaceAccess":   {"GET /api/v1/spaces/{space_id}/access", "access.explain", "rl.read"},
		"explainChannelAccess": {"GET /api/v1/channels/{channel_id}/access", "access.explain", "rl.read"},
		"previewGrant":         {"POST /api/v1/grants/preview", "access.preview", "rl.admin"},
		"viewAsUser":           {"GET /api/v1/users/{user_id}/view-as", "access.view_as", "rl.admin"},
		"revokeGrant":          {"POST /api/v1/grants/{grant_id}/revoke", "grant.revoke", "rl.admin"},
		// M1-51
		"listAgents":              {"GET /api/v1/agents", "agent.read", "rl.read"},
		"createAgent":             {"POST /api/v1/agents", "agent.create", "rl.admin"},
		"getAgent":                {"GET /api/v1/agents/{agent_id}", "agent.read", "rl.read"},
		"updateAgent":             {"PATCH /api/v1/agents/{agent_id}", "agent.update", "rl.admin"},
		"disableAgent":            {"POST /api/v1/agents/{agent_id}/disable", "agent.disable", "rl.admin"},
		"enableAgent":             {"POST /api/v1/agents/{agent_id}/enable", "agent.enable", "rl.admin"},
		"setAgentBillingMode":     {"PUT /api/v1/agents/{agent_id}/billing-mode", "agent.billing_mode.set", "rl.admin"},
		"setAllAgentsBillingMode": {"PUT /api/v1/instance/agent-billing-mode", "agent.billing_mode.set_all", "rl.admin"},
		"setCredentialHolder":     {"PUT /api/v1/agents/{agent_id}/credential-holder", "agent.credential_holder.set", "rl.admin"},
		"setAgentToolClass":       {"PUT /api/v1/agents/{agent_id}/tool-class", "agent.tool_class.set", "rl.admin"},
		// M2-07
		"getConsumerState": {"GET /api/v1/agents/me/consumers/{consumer}", "agent.events.consume", "rl.agent_runtime"},
		"pollAgentEvents":  {"GET /api/v1/agents/me/consumers/{consumer}/events", "agent.events.consume", "rl.agent_runtime"},
		"ackAgentEvents":   {"POST /api/v1/agents/me/consumers/{consumer}/ack", "agent.events.consume", "rl.agent_runtime"},
		// M1-45
		"disableUser":   {"POST /api/v1/users/{user_id}/disable", "user.disable", "rl.admin"},
		"revokeDevice":  {"POST /api/v1/devices/{device_id}/revoke", "device.revoke", "rl.admin"},
		"revokeSession": {"POST /api/v1/sessions/{session_id}/revoke", "session.revoke", "rl.admin"},
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
		// M1-39
		"loginWithPassword":    {"POST /api/v1/auth/password/login", "auth.login.complete", "rl.public_auth"},
		"setMyLocalCredential": {"PUT /api/v1/me/local-credential", "local_credential.set", "rl.admin"},
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
		// M1-52
		"listSpaces":        {"GET /api/v1/spaces", "space.read", "rl.read"},
		"createSpace":       {"POST /api/v1/spaces", "space.create", "rl.admin"},
		"getSpace":          {"GET /api/v1/spaces/{space_id}", "space.read", "rl.read"},
		"updateSpace":       {"PATCH /api/v1/spaces/{space_id}", "space.update", "rl.admin"},
		"moveSpace":         {"POST /api/v1/spaces/{space_id}/move", "space.reorder", "rl.admin"},
		"archiveSpace":      {"POST /api/v1/spaces/{space_id}/archive", "space.archive", "rl.admin"},
		"putMySidebarSpace": {"PUT /api/v1/me/sidebar/spaces/{space_id}", "sidebar.update", "rl.write"},
		// M1-53
		"listSpaceChannels": {"GET /api/v1/spaces/{space_id}/channels", "channel.read", "rl.read"},
		"createChannel":     {"POST /api/v1/spaces/{space_id}/channels", "channel.create", "rl.admin"},
		"getChannel":        {"GET /api/v1/channels/{channel_id}", "channel.read", "rl.read"},
		"updateChannel":     {"PATCH /api/v1/channels/{channel_id}", "channel.update", "rl.admin"},
		"moveChannel":       {"POST /api/v1/channels/{channel_id}/move", "channel.reorder", "rl.admin"},
		"archiveChannel":    {"POST /api/v1/channels/{channel_id}/archive", "channel.archive", "rl.admin"},
		"listForumTags":     {"GET /api/v1/channels/{channel_id}/tags", "channel.read", "rl.read"},
		"createForumTag":    {"POST /api/v1/channels/{channel_id}/tags", "forum_tag.manage", "rl.admin"},
		"updateForumTag":    {"PATCH /api/v1/channels/{channel_id}/tags/{tag_id}", "forum_tag.manage", "rl.admin"},
		"moveForumTag":      {"POST /api/v1/channels/{channel_id}/tags/{tag_id}/move", "forum_tag.manage", "rl.admin"},
		"deleteForumTag":    {"DELETE /api/v1/channels/{channel_id}/tags/{tag_id}", "forum_tag.manage", "rl.admin"},
		// M1-54
		"setSpaceOperator":   {"PUT /api/v1/spaces/{space_id}/operator", "space.operator.set", "rl.admin"},
		"setChannelOperator": {"PUT /api/v1/channels/{channel_id}/operator", "channel.operator.set", "rl.admin"},
		// M1-55
		"listChannelThreads": {"GET /api/v1/channels/{channel_id}/threads", "thread.read", "rl.read"},
		"getThread":          {"GET /api/v1/threads/{thread_id}", "thread.read", "rl.read"},
		"setThreadTitle":     {"PUT /api/v1/threads/{thread_id}/title", "thread.title.update", "rl.write"},
		"closeThread":        {"POST /api/v1/threads/{thread_id}/close", "thread.close", "rl.write"},
		"reopenThread":       {"POST /api/v1/threads/{thread_id}/reopen", "thread.reopen", "rl.write"},
		"setThreadTags":      {"PUT /api/v1/threads/{thread_id}/tags", "thread.tags.set", "rl.write"},
		// M1-62
		"moveThread": {"POST /api/v1/threads/{thread_id}/move", "thread.move", "rl.admin"},
		// M1-56
		"createMessage":          {"POST /api/v1/threads/{thread_id}/messages", "message.create", "rl.message"},
		"createThread":           {"POST /api/v1/channels/{channel_id}/threads", "thread.create", "rl.message"},
		"listMyMentions":         {"GET /api/v1/me/mentions", "mention.list", "rl.read"},
		"listMyTemplates":        {"GET /api/v1/me/templates", "template.read", "rl.read"},
		"createPersonalTemplate": {"POST /api/v1/me/templates", "template.manage", "rl.write"},
		"listTemplates":          {"GET /api/v1/channels/{channel_id}/templates", "template.read", "rl.read"},
		"createChannelTemplate":  {"POST /api/v1/channels/{channel_id}/templates", "template.manage", "rl.write"},
		"updateTemplate":         {"PATCH /api/v1/templates/{template_id}", "template.manage", "rl.write"},
		"deleteTemplate":         {"DELETE /api/v1/templates/{template_id}", "template.manage", "rl.write"},
		// M1-57
		"listThreadMessages": {"GET /api/v1/threads/{thread_id}/messages", "thread.read", "rl.read"},
		"getMessage":         {"GET /api/v1/messages/{message_id}", "message.read", "rl.read"},
		// M1-58
		"updateMessage":        {"PATCH /api/v1/messages/{message_id}", "message.update", "rl.message"},
		"deleteMessage":        {"DELETE /api/v1/messages/{message_id}", "message.delete", "rl.message"},
		"listMessageRevisions": {"GET /api/v1/messages/{message_id}/revisions", "message.revisions.read", "rl.read"},
		// M1-59
		"addReaction":    {"PUT /api/v1/messages/{message_id}/reactions/{emoji}", "reaction.add", "rl.write"},
		"removeReaction": {"DELETE /api/v1/messages/{message_id}/reactions/{emoji}", "reaction.remove", "rl.write"},
		"listReactions":  {"GET /api/v1/messages/{message_id}/reactions", "message.read", "rl.read"},
		"pinMessage":     {"PUT /api/v1/messages/{message_id}/pin", "pin.create", "rl.write"},
		"unpinMessage":   {"DELETE /api/v1/messages/{message_id}/pin", "pin.delete", "rl.write"},
		"listPins":       {"GET /api/v1/threads/{thread_id}/pins", "thread.read", "rl.read"},
		// M1-61
		"listMyReadStates":   {"GET /api/v1/me/read-states", "settings.read", "rl.read"},
		"putReadState":       {"PUT /api/v1/threads/{thread_id}/read-state", "read_state.update", "rl.write"},
		"markThreadUnread":   {"POST /api/v1/threads/{thread_id}/read-state/mark-unread", "read_state.mark_unread", "rl.write"},
		"putThreadDraft":     {"PUT /api/v1/threads/{thread_id}/draft", "draft.write", "rl.write"},
		"deleteThreadDraft":  {"DELETE /api/v1/threads/{thread_id}/draft", "draft.write", "rl.write"},
		"putChannelDraft":    {"PUT /api/v1/channels/{channel_id}/draft", "draft.write", "rl.write"},
		"deleteChannelDraft": {"DELETE /api/v1/channels/{channel_id}/draft", "draft.write", "rl.write"},
		"listMyDrafts":       {"GET /api/v1/me/drafts", "draft.read", "rl.read"},
		"listMySettings":     {"GET /api/v1/me/settings", "settings.read", "rl.read"},
		"putMySetting":       {"PUT /api/v1/me/settings/{key}", "settings.update", "rl.write"},
		"deleteMySetting":    {"DELETE /api/v1/me/settings/{key}", "settings.update", "rl.write"},
		"getMySidebar":       {"GET /api/v1/me/sidebar", "settings.read", "rl.read"},
		// M3-18
		"listMyNotificationPreferences": {"GET /api/v1/me/notification-preferences", "settings.read", "rl.read"},
		"putNotificationPreference":     {"PUT /api/v1/me/notification-preferences/{target_kind}/{target_id}", "notification_pref.update", "rl.write"},
		"deleteNotificationPreference":  {"DELETE /api/v1/me/notification-preferences/{target_kind}/{target_id}", "notification_pref.update", "rl.write"},
		"listMyQuietHours":              {"GET /api/v1/me/quiet-hours", "settings.read", "rl.read"},
		"createQuietHoursRule":          {"POST /api/v1/me/quiet-hours", "quiet_hours.update", "rl.write"},
		"putQuietHoursRule":             {"PUT /api/v1/me/quiet-hours/{rule_id}", "quiet_hours.update", "rl.write"},
		"deleteQuietHoursRule":          {"DELETE /api/v1/me/quiet-hours/{rule_id}", "quiet_hours.update", "rl.write"},
		// M2-39
		"listHosts":             {"GET /api/v1/hosts", "host.read", "rl.read"},
		"createHost":            {"POST /api/v1/hosts", "host.manage", "rl.admin"},
		"getHost":               {"GET /api/v1/hosts/{host_id}", "host.read", "rl.read"},
		"updateHost":            {"PATCH /api/v1/hosts/{host_id}", "host.manage", "rl.admin"},
		"setHostState":          {"POST /api/v1/hosts/{host_id}/state", "host.manage", "rl.admin"},
		"setHostLlmCredentials": {"PUT /api/v1/hosts/{host_id}/llm-credentials", "host.manage", "rl.admin"},
		// M2-41
		"issueHostPairingCode": {"POST /api/v1/hosts/{host_id}/pairing-code", "host.manage", "rl.admin"},
		"pairHost":             {"POST /api/v1/hosts/pair", "host.pair", "rl.public_auth"},
		"confirmHost":          {"POST /api/v1/hosts/{host_id}/confirm", "host.manage", "rl.admin"},
		// M2-24
		"listApps":           {"GET /api/v1/apps", "app.read", "rl.read"},
		"installApp":         {"POST /api/v1/apps", "app.install", "rl.admin"},
		"validateManifest":   {"POST /api/v1/app-manifests/validate", "app.read", "rl.read"},
		"getApp":             {"GET /api/v1/apps/{app_id}", "app.read", "rl.read"},
		"upgradeAppManifest": {"PUT /api/v1/apps/{app_id}/manifest", "app.install", "rl.admin"},
		"setAppScopes":       {"PUT /api/v1/apps/{app_id}/scopes", "app.scopes.update", "rl.admin"},
		"suspendApp":         {"POST /api/v1/apps/{app_id}/suspend", "app.suspend", "rl.admin"},
		"resumeApp":          {"POST /api/v1/apps/{app_id}/resume", "app.resume", "rl.admin"},
		"uninstallApp":       {"POST /api/v1/apps/{app_id}/uninstall", "app.uninstall", "rl.admin"},
		// M2-26
		"getWebhookKeys":            {"GET /api/v1/instance/webhook-keys", "instance.discovery.read", "rl.public_auth"},
		"listWebhookSubscriptions":  {"GET /api/v1/apps/{app_id}/webhooks", "webhook_out.read", "rl.read"},
		"createWebhookSubscription": {"POST /api/v1/apps/{app_id}/webhooks", "webhook_out.manage", "rl.admin"},
		"updateWebhookSubscription": {"PATCH /api/v1/webhooks/{subscription_id}", "webhook_out.manage", "rl.admin"},
		"deleteWebhookSubscription": {"DELETE /api/v1/webhooks/{subscription_id}", "webhook_out.manage", "rl.admin"},
		"listWebhookDeliveries":     {"GET /api/v1/webhooks/{subscription_id}/deliveries", "webhook_out.read", "rl.read"},
		"redeliverWebhook":          {"POST /api/v1/webhook-deliveries/{delivery_id}/redeliver", "webhook_out.redeliver", "rl.admin"},
		// M1-63
		"listEvents":  {"GET /api/v1/events", "events.catch_up", "rl.read"},
		"getSnapshot": {"GET /api/v1/sync/snapshot", "sync.snapshot.read", "rl.read"},
		// M6-01
		"getConsoleSession":        {"GET /api/v1/console/session", "self.read", "rl.read"},
		"endViewAs":                {"DELETE /api/v1/console/session/view-as", "access.view_as.end", "rl.admin"},
		"stepUpWithPassword":       {"POST /api/v1/auth/step-up/password", "auth.step_up", "rl.public_auth"},
		"beginStepUpOidcReauth":    {"POST /api/v1/auth/step-up/oidc", "auth.step_up", "rl.public_auth"},
		"completeStepUpOidcReauth": {"POST /api/v1/auth/step-up/oidc/complete", "auth.step_up", "rl.public_auth"},
		// M2-02
		"getChannelBehavior":     {"GET /api/v1/channels/{channel_id}/behavior", "channel.behavior.read", "rl.read"},
		"putChannelBehavior":     {"PUT /api/v1/channels/{channel_id}/behavior", "channel.behavior.update", "rl.admin"},
		"listServedChannels":     {"GET /api/v1/agents/me/channels", "agent.config.read", "rl.read"},
		"getServedChannelConfig": {"GET /api/v1/agents/me/channels/{channel_id}/config", "agent.config.read", "rl.read"},
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
	if _, ok := routes.ByID("deleteFile"); ok {
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
