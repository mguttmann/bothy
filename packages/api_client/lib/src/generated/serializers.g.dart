// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'serializers.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

Serializers _$serializers =
    (Serializers().toBuilder()
          ..add($EventEnvelope.serializer)
          ..add($PushRegistration.serializer)
          ..add(AccessChangedEvent.serializer)
          ..add(AccessChangedEventTypeEnum.serializer)
          ..add(AccessChangedPayload.serializer)
          ..add(AccessEffect.serializer)
          ..add(AccessEntry.serializer)
          ..add(AccessExpiringEvent.serializer)
          ..add(AccessExpiringEventTypeEnum.serializer)
          ..add(AccessExpiringPayload.serializer)
          ..add(AccessExpiringPayloadKindEnum.serializer)
          ..add(AccessExplanation.serializer)
          ..add(AccessLabel.serializer)
          ..add(AccessLabels.serializer)
          ..add(AccessSource.serializer)
          ..add(AccessSourceKindEnum.serializer)
          ..add(AccessSourceRuleEnum.serializer)
          ..add(AccessTarget.serializer)
          ..add(AccessTargetKind.serializer)
          ..add(ActorKind.serializer)
          ..add(Agent.serializer)
          ..add(AgentChannelConfig.serializer)
          ..add(AgentChannelConfigList.serializer)
          ..add(AgentConfigChangedEvent.serializer)
          ..add(AgentConfigChangedEventTypeEnum.serializer)
          ..add(AgentConfigChangedPayload.serializer)
          ..add(AgentCreate.serializer)
          ..add(AgentFollowupEvent.serializer)
          ..add(AgentFollowupEventTypeEnum.serializer)
          ..add(AgentFollowupPayload.serializer)
          ..add(AgentInvocationEvent.serializer)
          ..add(AgentInvocationEventTypeEnum.serializer)
          ..add(AgentInvocationPayload.serializer)
          ..add(AgentInvocationPayloadSessionModeEnum.serializer)
          ..add(AgentInvocationPayloadTriggerKindEnum.serializer)
          ..add(AgentPage.serializer)
          ..add(AgentPayload.serializer)
          ..add(AgentPresence.serializer)
          ..add(AgentPresenceChangedEvent.serializer)
          ..add(AgentPresenceChangedEventTypeEnum.serializer)
          ..add(AgentPresencePayload.serializer)
          ..add(AgentRateLimitState.serializer)
          ..add(AgentRateLimitStateStateEnum.serializer)
          ..add(AgentRuntimeState.serializer)
          ..add(AgentRuntimeStatus.serializer)
          ..add(AgentSession.serializer)
          ..add(AgentSessionBind.serializer)
          ..add(AgentSessionChangedEvent.serializer)
          ..add(AgentSessionChangedEventTypeEnum.serializer)
          ..add(AgentSessionChangedPayload.serializer)
          ..add(AgentSessionEndReasonEnum.serializer)
          ..add(AgentSessionList.serializer)
          ..add(AgentSessionReport.serializer)
          ..add(AgentSessionReportBindFailedEnum.serializer)
          ..add(AgentStatusEnum.serializer)
          ..add(AgentTriggerers.serializer)
          ..add(AgentUpdate.serializer)
          ..add(AgentUpdatedEvent.serializer)
          ..add(AgentUpdatedEventTypeEnum.serializer)
          ..add(AlertmanagerAlert.serializer)
          ..add(AlertmanagerAlertStatusEnum.serializer)
          ..add(ApiToken.serializer)
          ..add(ApiTokenCreate.serializer)
          ..add(ApiTokenCreated.serializer)
          ..add(ApiTokenPage.serializer)
          ..add(App.serializer)
          ..add(AppInstall.serializer)
          ..add(AppInstallation.serializer)
          ..add(AppInstallationStateEnum.serializer)
          ..add(AppLockPolicy.serializer)
          ..add(AppManifest.serializer)
          ..add(AppManifestComponentsEnum.serializer)
          ..add(AppManifestDelivery.serializer)
          ..add(AppManifestDeliveryModeEnum.serializer)
          ..add(AppManifestIncomingWebhooksInner.serializer)
          ..add(AppManifestRequestedGrantsInner.serializer)
          ..add(AppManifestSchemaVersionEnum.serializer)
          ..add(AppManifestScopes.serializer)
          ..add(AppPage.serializer)
          ..add(AppScope.serializer)
          ..add(AppScopesPut.serializer)
          ..add(Attachment.serializer)
          ..add(AuditDecision.serializer)
          ..add(AuditEvent.serializer)
          ..add(AuditEventPage.serializer)
          ..add(AuditVerifyRequest.serializer)
          ..add(AuditVerifyResult.serializer)
          ..add(AuditVerifyResultFailureReasonEnum.serializer)
          ..add(AuthProviderInfo.serializer)
          ..add(AutocompleteAnswer.serializer)
          ..add(AutocompleteQuery.serializer)
          ..add(AutocompleteResult.serializer)
          ..add(BillingEvidence.serializer)
          ..add(BillingEvidenceKeyTypeEnum.serializer)
          ..add(BillingMode.serializer)
          ..add(BillingModeSet.serializer)
          ..add(BillingModeSetAll.serializer)
          ..add(BillingModeSetAllResult.serializer)
          ..add(Branding.serializer)
          ..add(BrandingUpdate.serializer)
          ..add(Capability.serializer)
          ..add(Channel.serializer)
          ..add(ChannelArchivedEvent.serializer)
          ..add(ChannelArchivedEventTypeEnum.serializer)
          ..add(ChannelBehavior.serializer)
          ..add(ChannelBehaviorPut.serializer)
          ..add(ChannelCreate.serializer)
          ..add(ChannelCreateKindEnum.serializer)
          ..add(ChannelCreatedEvent.serializer)
          ..add(ChannelCreatedEventTypeEnum.serializer)
          ..add(ChannelKind.serializer)
          ..add(ChannelLabel.serializer)
          ..add(ChannelOperator.serializer)
          ..add(ChannelPayload.serializer)
          ..add(ChannelReadOnlySet.serializer)
          ..add(ChannelSettingsChangedEvent.serializer)
          ..add(ChannelSettingsChangedEventTypeEnum.serializer)
          ..add(ChannelSettingsChangedPayload.serializer)
          ..add(ChannelUpdate.serializer)
          ..add(ChannelUpdatedEvent.serializer)
          ..add(ChannelUpdatedEventTypeEnum.serializer)
          ..add(ChannelView.serializer)
          ..add(ChannelViewList.serializer)
          ..add(ClientMetric.serializer)
          ..add(ClientMetricBucketsInner.serializer)
          ..add(ClientMetricKindEnum.serializer)
          ..add(ClientMetricsBatch.serializer)
          ..add(ClientMetricsBatchDensityEnum.serializer)
          ..add(ClientMetricsBatchPlatformEnum.serializer)
          ..add(ClientMetricsBatchThemeEnum.serializer)
          ..add(ClientVersionRequirement.serializer)
          ..add(Command.serializer)
          ..add(CommandAutocompleteRequestedEvent.serializer)
          ..add(CommandAutocompleteRequestedEventTypeEnum.serializer)
          ..add(CommandAutocompleteRequestedPayload.serializer)
          ..add(CommandChangedEvent.serializer)
          ..add(CommandChangedEventTypeEnum.serializer)
          ..add(CommandChangedPayload.serializer)
          ..add(CommandChangedPayloadChangeEnum.serializer)
          ..add(CommandChoice.serializer)
          ..add(CommandInvocation.serializer)
          ..add(CommandInvoke.serializer)
          ..add(CommandInvokedEvent.serializer)
          ..add(CommandInvokedEventTypeEnum.serializer)
          ..add(CommandInvokedPayload.serializer)
          ..add(CommandLevel.serializer)
          ..add(CommandList.serializer)
          ..add(CommandOption.serializer)
          ..add(CommandOptionTypeEnum.serializer)
          ..add(CommandOptionValue.serializer)
          ..add(CommandRegister.serializer)
          ..add(Component.serializer)
          ..add(ComponentCreate.serializer)
          ..add(ComponentRestriction.serializer)
          ..add(ComponentStateEnum.serializer)
          ..add(ComponentType.serializer)
          ..add(ComponentUpdate.serializer)
          ..add(ComponentUpdateStateEnum.serializer)
          ..add(ConsoleSession.serializer)
          ..add(ConsoleSessionStepUpMethodEnum.serializer)
          ..add(ConsoleSessionStepUpMethodsAvailableEnum.serializer)
          ..add(ConsoleSessionViewAs.serializer)
          ..add(ConsumerAck.serializer)
          ..add(ConsumerState.serializer)
          ..add(CostBreakdown.serializer)
          ..add(CostBucket.serializer)
          ..add(CostEstimate.serializer)
          ..add(CostEstimatePage.serializer)
          ..add(CostReconcile.serializer)
          ..add(CostReconcileResult.serializer)
          ..add(CostReport.serializer)
          ..add(CostSeries.serializer)
          ..add(CostTotal.serializer)
          ..add(CredentialHolderSet.serializer)
          ..add(CredentialOverview.serializer)
          ..add(CspReportLegacy.serializer)
          ..add(Device.serializer)
          ..add(DeviceApprove.serializer)
          ..add(DeviceKeyRegistration.serializer)
          ..add(DeviceKeyRegistrationAlgEnum.serializer)
          ..add(DevicePage.serializer)
          ..add(DevicePendingEvent.serializer)
          ..add(DevicePendingEventTypeEnum.serializer)
          ..add(DevicePendingPayload.serializer)
          ..add(DevicePlatform.serializer)
          ..add(DeviceRegistration.serializer)
          ..add(DeviceRegistrationAlgEnum.serializer)
          ..add(DeviceRevokeReasonEnum.serializer)
          ..add(DeviceStateEnum.serializer)
          ..add(Discovery.serializer)
          ..add(DmCreate.serializer)
          ..add(Draft.serializer)
          ..add(DraftDeletedEvent.serializer)
          ..add(DraftDeletedEventTypeEnum.serializer)
          ..add(DraftDeletedPayload.serializer)
          ..add(DraftList.serializer)
          ..add(DraftPayload.serializer)
          ..add(DraftPut.serializer)
          ..add(DraftPutResult.serializer)
          ..add(DraftRef.serializer)
          ..add(DraftUpdatedEvent.serializer)
          ..add(DraftUpdatedEventTypeEnum.serializer)
          ..add(EnrollBeginRequest.serializer)
          ..add(EnrollCompleteRequest.serializer)
          ..add(EnrollRedeemRequest.serializer)
          ..add(EnrollmentCode.serializer)
          ..add(EnrollmentCodePurposeEnum.serializer)
          ..add(EvaluatedFlag.serializer)
          ..add(Event.serializer)
          ..add(EventActor.serializer)
          ..add(EventEnvelopeAudienceEnum.serializer)
          ..add(EventPage.serializer)
          ..add(EventResource.serializer)
          ..add(EventScope.serializer)
          ..add(ExpiringAccessItem.serializer)
          ..add(ExpiringAccessItemKindEnum.serializer)
          ..add(ExpiringAccessPage.serializer)
          ..add(ExtensionSessionCreate.serializer)
          ..add(ExtensionSessionCreateClientComponentEnum.serializer)
          ..add(FeatureFlag.serializer)
          ..add(FeatureFlagKindEnum.serializer)
          ..add(FeatureFlagList.serializer)
          ..add(FeatureFlagPut.serializer)
          ..add(FieldError.serializer)
          ..add(File.serializer)
          ..add(FilePayload.serializer)
          ..add(FilePurposeEnum.serializer)
          ..add(FileScanStatusEnum.serializer)
          ..add(FileTicket.serializer)
          ..add(FileTicketCreate.serializer)
          ..add(FileTicketCreateVariantEnum.serializer)
          ..add(FileUpdatedEvent.serializer)
          ..add(FileUpdatedEventTypeEnum.serializer)
          ..add(FileUploadStateEnum.serializer)
          ..add(FlagRollout.serializer)
          ..add(ForumSort.serializer)
          ..add(ForumTag.serializer)
          ..add(ForumTagCreate.serializer)
          ..add(ForumTagList.serializer)
          ..add(ForumTagUpdate.serializer)
          ..add(GatewayAck.serializer)
          ..add(GatewayAutocompleteResult.serializer)
          ..add(GatewayCatchupComplete.serializer)
          ..add(GatewayClientInfo.serializer)
          ..add(GatewayCursor.serializer)
          ..add(GatewayError.serializer)
          ..add(GatewayFocus.serializer)
          ..add(GatewayHeartbeat.serializer)
          ..add(GatewayHeartbeatAck.serializer)
          ..add(GatewayHello.serializer)
          ..add(GatewayIdentify.serializer)
          ..add(GatewayInfo.serializer)
          ..add(GatewayReady.serializer)
          ..add(GatewayReauth.serializer)
          ..add(GatewayReauthRequired.serializer)
          ..add(GatewayResume.serializer)
          ..add(GatewayResumed.serializer)
          ..add(GatewayResyncRequired.serializer)
          ..add(GatewayResyncRequiredReasonEnum.serializer)
          ..add(GatewayRunUpdate.serializer)
          ..add(GatewayStatsSubscribe.serializer)
          ..add(GatewayTyping.serializer)
          ..add(GdprDecision.serializer)
          ..add(GdprDecisionDecisionEnum.serializer)
          ..add(GdprDecisionExtendMonthsEnum.serializer)
          ..add(GdprRequest.serializer)
          ..add(GdprRequestCreate.serializer)
          ..add(GdprRequestDueEvent.serializer)
          ..add(GdprRequestDueEventTypeEnum.serializer)
          ..add(GdprRequestDuePayload.serializer)
          ..add(GdprRequestKind.serializer)
          ..add(GdprRequestPage.serializer)
          ..add(GdprRequestPayload.serializer)
          ..add(GdprRequestState.serializer)
          ..add(GdprRequestUpdatedEvent.serializer)
          ..add(GdprRequestUpdatedEventTypeEnum.serializer)
          ..add(Grant.serializer)
          ..add(GrantChangedEvent.serializer)
          ..add(GrantChangedEventTypeEnum.serializer)
          ..add(GrantChangedPayload.serializer)
          ..add(GrantChangedPayloadChangeEnum.serializer)
          ..add(GrantCreate.serializer)
          ..add(GrantPage.serializer)
          ..add(GrantPreview.serializer)
          ..add(GrantPreviewTextLocaleEnum.serializer)
          ..add(GrantSubjectKind.serializer)
          ..add(Group.serializer)
          ..add(GroupChangedEvent.serializer)
          ..add(GroupChangedEventTypeEnum.serializer)
          ..add(GroupChangedPayload.serializer)
          ..add(GroupChangedPayloadChangeEnum.serializer)
          ..add(GroupCreate.serializer)
          ..add(GroupMember.serializer)
          ..add(GroupMemberAdd.serializer)
          ..add(GroupMemberPage.serializer)
          ..add(GroupPage.serializer)
          ..add(GroupSource.serializer)
          ..add(GroupUpdate.serializer)
          ..add(HeaderPair.serializer)
          ..add(Heartbeat.serializer)
          ..add(HeartbeatAck.serializer)
          ..add(HeartbeatStateEnum.serializer)
          ..add(Host.serializer)
          ..add(HostChannelHello.serializer)
          ..add(HostConfirm.serializer)
          ..add(HostCreate.serializer)
          ..add(HostDisabledReasonEnum.serializer)
          ..add(HostDoctorCheck.serializer)
          ..add(HostDoctorCheckCheckEnum.serializer)
          ..add(HostHeartbeat.serializer)
          ..add(HostHeartbeatAck.serializer)
          ..add(HostIdentify.serializer)
          ..add(HostLlmCredentialsSet.serializer)
          ..add(HostPage.serializer)
          ..add(HostPairRequest.serializer)
          ..add(HostPairResult.serializer)
          ..add(HostPairingCode.serializer)
          ..add(HostPlatform.serializer)
          ..add(HostPresenceChangedEvent.serializer)
          ..add(HostPresenceChangedEventTypeEnum.serializer)
          ..add(HostPresencePayload.serializer)
          ..add(HostProfile.serializer)
          ..add(HostProfileKindEnum.serializer)
          ..add(HostReady.serializer)
          ..add(HostReport.serializer)
          ..add(HostReportedSessionUser.serializer)
          ..add(HostSessionClose.serializer)
          ..add(HostSessionEnded.serializer)
          ..add(HostSessionOpen.serializer)
          ..add(HostSessionStarted.serializer)
          ..add(HostSessionUser.serializer)
          ..add(HostSessionUserAssign.serializer)
          ..add(HostState.serializer)
          ..add(HostStateSet.serializer)
          ..add(HostStateSetStateEnum.serializer)
          ..add(HostUpdate.serializer)
          ..add(HostUpdatedEvent.serializer)
          ..add(HostUpdatedEventTypeEnum.serializer)
          ..add(HostUpdatedPayload.serializer)
          ..add(Identity.serializer)
          ..add(IdentityLinkBegin.serializer)
          ..add(IdentityLinkComplete.serializer)
          ..add(IdentityProvider.serializer)
          ..add(IdentityProviderCreate.serializer)
          ..add(IdentityProviderKind.serializer)
          ..add(IdentityProviderList.serializer)
          ..add(IdentityProviderUpdate.serializer)
          ..add(ImportBatch.serializer)
          ..add(ImportBatchResult.serializer)
          ..add(ImportChannelData.serializer)
          ..add(ImportChannelDataKindEnum.serializer)
          ..add(ImportCursorSet.serializer)
          ..add(ImportEarlierSession.serializer)
          ..add(ImportFileCreate.serializer)
          ..add(ImportFileSlot.serializer)
          ..add(ImportForumTagData.serializer)
          ..add(ImportItem.serializer)
          ..add(ImportItemItemKindEnum.serializer)
          ..add(ImportItemPage.serializer)
          ..add(ImportItemSessionEnum.serializer)
          ..add(ImportItemState.serializer)
          ..add(ImportKind.serializer)
          ..add(ImportLeadSet.serializer)
          ..add(ImportLeadSetLeadEnum.serializer)
          ..add(ImportMessageData.serializer)
          ..add(ImportObject.serializer)
          ..add(ImportObjectItemKindEnum.serializer)
          ..add(ImportPinData.serializer)
          ..add(ImportProgressEvent.serializer)
          ..add(ImportProgressEventTypeEnum.serializer)
          ..add(ImportProgressPayload.serializer)
          ..add(ImportSource.serializer)
          ..add(ImportSourceCreate.serializer)
          ..add(ImportSourceDiscordConfig.serializer)
          ..add(ImportSourceDiscordConfigChannelsInner.serializer)
          ..add(ImportSourceDiscordConfigChannelsInnerLeadEnum.serializer)
          ..add(ImportSourceDiscordConfigDefaults.serializer)
          ..add(ImportSourceDiscordConfigDefaultsUnmappedBotEnum.serializer)
          ..add(ImportSourceDiscordConfigDefaultsUnmappedHumanEnum.serializer)
          ..add(ImportSourceDiscordConfigSpacesInner.serializer)
          ..add(ImportSourceDiscordConfigUsersInner.serializer)
          ..add(ImportSourceDiscordConfigVersionEnum.serializer)
          ..add(ImportSourcePage.serializer)
          ..add(ImportSourceStateEnum.serializer)
          ..add(ImportSpaceData.serializer)
          ..add(ImportThreadData.serializer)
          ..add(ImportUserData.serializer)
          ..add(ImportUserDataDiscordKindEnum.serializer)
          ..add(IncomingWebhook.serializer)
          ..add(IncomingWebhookCreate.serializer)
          ..add(IncomingWebhookCreated.serializer)
          ..add(IncomingWebhookList.serializer)
          ..add(IncomingWebhookPost.serializer)
          ..add(IncomingWebhookPostStatusEnum.serializer)
          ..add(IncomingWebhookPostVersionEnum.serializer)
          ..add(IncomingWebhookResult.serializer)
          ..add(InstanceInfo.serializer)
          ..add(InstanceLimits.serializer)
          ..add(InstanceSetting.serializer)
          ..add(InstanceSettingList.serializer)
          ..add(InstanceSettingPut.serializer)
          ..add(InstanceUpdatedEvent.serializer)
          ..add(InstanceUpdatedEventTypeEnum.serializer)
          ..add(InstanceUpdatedPayload.serializer)
          ..add(Interaction.serializer)
          ..add(InteractionCreate.serializer)
          ..add(InteractionCreatedEvent.serializer)
          ..add(InteractionCreatedEventTypeEnum.serializer)
          ..add(InteractionCreatedPayload.serializer)
          ..add(InteractionRespond.serializer)
          ..add(InteractionRespondStateEnum.serializer)
          ..add(InteractionResponse.serializer)
          ..add(InteractionStateEnum.serializer)
          ..add(InvariantViolation.serializer)
          ..add(InvocationTokenGrant.serializer)
          ..add(InvocationTokenRenew.serializer)
          ..add(JsonPatchOperation.serializer)
          ..add(JsonPatchOperationOpEnum.serializer)
          ..add(Jwks.serializer)
          ..add(Level.serializer)
          ..add(LevelOrNone.serializer)
          ..add(LlmCredentials.serializer)
          ..add(LocalCredentialResult.serializer)
          ..add(LocalCredentialSet.serializer)
          ..add(LoginClient.serializer)
          ..add(ManifestValidation.serializer)
          ..add(Me.serializer)
          ..add(MentionItem.serializer)
          ..add(MentionItemPage.serializer)
          ..add(Message.serializer)
          ..add(MessageCreate.serializer)
          ..add(MessageCreateKindEnum.serializer)
          ..add(MessageCreatedEvent.serializer)
          ..add(MessageCreatedEventTypeEnum.serializer)
          ..add(MessageDeletedEvent.serializer)
          ..add(MessageDeletedEventTypeEnum.serializer)
          ..add(MessageDeletedPayload.serializer)
          ..add(MessageKind.serializer)
          ..add(MessageOrigin.serializer)
          ..add(MessagePage.serializer)
          ..add(MessagePayload.serializer)
          ..add(MessageRevision.serializer)
          ..add(MessageRevisionList.serializer)
          ..add(MessageSystemCodeEnum.serializer)
          ..add(MessageUpdate.serializer)
          ..add(MessageUpdatedEvent.serializer)
          ..add(MessageUpdatedEventTypeEnum.serializer)
          ..add(Money.serializer)
          ..add(MovePosition.serializer)
          ..add(NotificationCreatedEvent.serializer)
          ..add(NotificationCreatedEventTypeEnum.serializer)
          ..add(NotificationCreatedPayload.serializer)
          ..add(NotificationKind.serializer)
          ..add(NotificationLevel.serializer)
          ..add(NotificationPrefPayload.serializer)
          ..add(NotificationPrefUpdatedEvent.serializer)
          ..add(NotificationPrefUpdatedEventTypeEnum.serializer)
          ..add(NotificationPreference.serializer)
          ..add(NotificationPreferenceList.serializer)
          ..add(NotificationPreferencePut.serializer)
          ..add(NotificationPreferenceTargetKindEnum.serializer)
          ..add(OidcBeginRequest.serializer)
          ..add(OidcBeginResponse.serializer)
          ..add(OidcCompleteRequest.serializer)
          ..add(OperatorMode.serializer)
          ..add(OperatorSet.serializer)
          ..add(Owner.serializer)
          ..add(OwnerAdd.serializer)
          ..add(OwnerList.serializer)
          ..add(PairingApprove.serializer)
          ..add(PairingBeginRequest.serializer)
          ..add(PairingCompleteRequest.serializer)
          ..add(PairingStarted.serializer)
          ..add(PairingStatus.serializer)
          ..add(PairingStatusStatusEnum.serializer)
          ..add(Passkey.serializer)
          ..add(PasskeyLoginBeginRequest.serializer)
          ..add(PasskeyLoginCompleteRequest.serializer)
          ..add(PasskeyRegisterComplete.serializer)
          ..add(PasswordLoginRequest.serializer)
          ..add(PermissionHint.serializer)
          ..add(PermissionHintKind.serializer)
          ..add(PermissionHintPage.serializer)
          ..add(PermissionHintRulesEnum.serializer)
          ..add(Pin.serializer)
          ..add(PinAddedEvent.serializer)
          ..add(PinAddedEventTypeEnum.serializer)
          ..add(PinList.serializer)
          ..add(PinPayload.serializer)
          ..add(PinRemovedEvent.serializer)
          ..add(PinRemovedEventTypeEnum.serializer)
          ..add(PreviewHint.serializer)
          ..add(PreviewHintCodeEnum.serializer)
          ..add(Price.serializer)
          ..add(PriceClose.serializer)
          ..add(PriceCreate.serializer)
          ..add(PriceList.serializer)
          ..add(PriceUnit.serializer)
          ..add(PrincipalKind.serializer)
          ..add(PrincipalRef.serializer)
          ..add(Problem.serializer)
          ..add(ProblemType.serializer)
          ..add(ProfileUpdate.serializer)
          ..add(ProjectBinding.serializer)
          ..add(ProjectBindingPut.serializer)
          ..add(ProjectReadiness.serializer)
          ..add(ProjectRunnerConfig.serializer)
          ..add(PublicBranding.serializer)
          ..add(PushEnvironment.serializer)
          ..add(PushPreview.serializer)
          ..add(PushProvider.serializer)
          ..add(PushRegistrationCreate.serializer)
          ..add(PushRegistrationCreated.serializer)
          ..add(QuietHoursList.serializer)
          ..add(QuietHoursPayload.serializer)
          ..add(QuietHoursRule.serializer)
          ..add(QuietHoursRuleInput.serializer)
          ..add(QuietHoursUpdatedEvent.serializer)
          ..add(QuietHoursUpdatedEventTypeEnum.serializer)
          ..add(Reaction.serializer)
          ..add(ReactionAddedEvent.serializer)
          ..add(ReactionAddedEventTypeEnum.serializer)
          ..add(ReactionPage.serializer)
          ..add(ReactionPayload.serializer)
          ..add(ReactionRemovedEvent.serializer)
          ..add(ReactionRemovedEventTypeEnum.serializer)
          ..add(ReactionSummary.serializer)
          ..add(ReadState.serializer)
          ..add(ReadStateList.serializer)
          ..add(ReadStatePayload.serializer)
          ..add(ReadStatePut.serializer)
          ..add(ReadStateUpdatedEvent.serializer)
          ..add(ReadStateUpdatedEventTypeEnum.serializer)
          ..add(ReasonBody.serializer)
          ..add(ReconcileEntry.serializer)
          ..add(ResolveI1.serializer)
          ..add(Run.serializer)
          ..add(RunApiRetry.serializer)
          ..add(RunAwaitInput.serializer)
          ..add(RunCancel.serializer)
          ..add(RunCancelRequestedEvent.serializer)
          ..add(RunCancelRequestedEventTypeEnum.serializer)
          ..add(RunCancelRequestedPayload.serializer)
          ..add(RunCancelRequestedPayloadReasonEnum.serializer)
          ..add(RunContext.serializer)
          ..add(RunContextAttachment.serializer)
          ..add(RunContextMessage.serializer)
          ..add(RunContextMessageRoleEnum.serializer)
          ..add(RunContextMessageTrustEnum.serializer)
          ..add(RunCost.serializer)
          ..add(RunFinish.serializer)
          ..add(RunFinishStateEnum.serializer)
          ..add(RunFollowup.serializer)
          ..add(RunFollowupStateEnum.serializer)
          ..add(RunMetrics.serializer)
          ..add(RunPatch.serializer)
          ..add(RunPatchResult.serializer)
          ..add(RunPlan.serializer)
          ..add(RunPlanItemsInner.serializer)
          ..add(RunPlanItemsInnerStatusEnum.serializer)
          ..add(RunStart.serializer)
          ..add(RunState.serializer)
          ..add(RunStateChangedEvent.serializer)
          ..add(RunStateChangedEventTypeEnum.serializer)
          ..add(RunStateChangedPayload.serializer)
          ..add(RunStep.serializer)
          ..add(RunStepKindEnum.serializer)
          ..add(RunStepPage.serializer)
          ..add(RunStepStatusEnum.serializer)
          ..add(RunSummary.serializer)
          ..add(RunSummaryPage.serializer)
          ..add(RunTriggerKindEnum.serializer)
          ..add(RunUpdatedEvent.serializer)
          ..add(RunUpdatedEventTypeEnum.serializer)
          ..add(RunUpdatedPayload.serializer)
          ..add(SearchHit.serializer)
          ..add(SearchHitKind.serializer)
          ..add(SearchHitPage.serializer)
          ..add(SearchHitSegmentsInner.serializer)
          ..add(Session.serializer)
          ..add(SessionAuthMethodEnum.serializer)
          ..add(SessionClientComponentEnum.serializer)
          ..add(SessionPage.serializer)
          ..add(SessionRevokedEvent.serializer)
          ..add(SessionRevokedEventTypeEnum.serializer)
          ..add(SessionRevokedPayload.serializer)
          ..add(SessionRevokedPayloadReasonEnum.serializer)
          ..add(SessionStepUpMethodEnum.serializer)
          ..add(SettingsPayload.serializer)
          ..add(SettingsUpdatedEvent.serializer)
          ..add(SettingsUpdatedEventTypeEnum.serializer)
          ..add(SidebarPayload.serializer)
          ..add(SidebarState.serializer)
          ..add(SidebarStateList.serializer)
          ..add(SidebarStatePut.serializer)
          ..add(SidebarUpdatedEvent.serializer)
          ..add(SidebarUpdatedEventTypeEnum.serializer)
          ..add(SigningKeyPurpose.serializer)
          ..add(SigningKeyRotate.serializer)
          ..add(SigningKeyRotateModeEnum.serializer)
          ..add(SigningKeyRotation.serializer)
          ..add(Snapshot.serializer)
          ..add(Space.serializer)
          ..add(SpaceArchivedEvent.serializer)
          ..add(SpaceArchivedEventTypeEnum.serializer)
          ..add(SpaceCreate.serializer)
          ..add(SpaceCreatedEvent.serializer)
          ..add(SpaceCreatedEventTypeEnum.serializer)
          ..add(SpacePayload.serializer)
          ..add(SpaceUpdate.serializer)
          ..add(SpaceUpdatedEvent.serializer)
          ..add(SpaceUpdatedEventTypeEnum.serializer)
          ..add(SpaceView.serializer)
          ..add(SpaceViewList.serializer)
          ..add(StatsBackendAgent.serializer)
          ..add(StatsBackendCost.serializer)
          ..add(StatsBackendDenials.serializer)
          ..add(StatsBackendReport.serializer)
          ..add(StatsBucket.serializer)
          ..add(StatsGranularity.serializer)
          ..add(StatsLiveAgent.serializer)
          ..add(StatsLiveConnections.serializer)
          ..add(StatsLiveEvent.serializer)
          ..add(StatsLiveEventTypeEnum.serializer)
          ..add(StatsLiveFrame.serializer)
          ..add(StatsLiveRate.serializer)
          ..add(StatsLiveRun.serializer)
          ..add(StatsLiveRunStateEnum.serializer)
          ..add(StatsLiveSli.serializer)
          ..add(StatsLiveStep.serializer)
          ..add(StatsLiveStepKindEnum.serializer)
          ..add(StatsMetric.serializer)
          ..add(StatsScope.serializer)
          ..add(StatsScopeKindEnum.serializer)
          ..add(StatsSeries.serializer)
          ..add(StatsSloEntry.serializer)
          ..add(StatsSloReport.serializer)
          ..add(StatsSloReportMeasuredByEnum.serializer)
          ..add(StatsSloState.serializer)
          ..add(StatsTimeline.serializer)
          ..add(StatsTimelineBucket.serializer)
          ..add(StatsTimelineRun.serializer)
          ..add(StatusComponent.serializer)
          ..add(StatusComponentInput.serializer)
          ..add(StatusComponentState.serializer)
          ..add(StatusIncident.serializer)
          ..add(StatusIncidentCreate.serializer)
          ..add(StatusIncidentImpact.serializer)
          ..add(StatusIncidentKind.serializer)
          ..add(StatusIncidentState.serializer)
          ..add(StatusIncidentUpdate.serializer)
          ..add(StatusIncidentUpdateCreate.serializer)
          ..add(StatusPage.serializer)
          ..add(StatusUpdatedEvent.serializer)
          ..add(StatusUpdatedEventTypeEnum.serializer)
          ..add(StatusUpdatedPayload.serializer)
          ..add(StepUpChallenge.serializer)
          ..add(StepUpDeviceKeyRequest.serializer)
          ..add(StepUpOidcBeginRequest.serializer)
          ..add(StepUpPasskeyRequest.serializer)
          ..add(StepUpPasswordRequest.serializer)
          ..add(StepUpResult.serializer)
          ..add(TargetKind.serializer)
          ..add(Template.serializer)
          ..add(TemplateCreate.serializer)
          ..add(TemplateList.serializer)
          ..add(TemplateScopeEnum.serializer)
          ..add(TemplateUpdate.serializer)
          ..add(TerminalCloseDetail.serializer)
          ..add(TerminalCloseReason.serializer)
          ..add(TerminalSession.serializer)
          ..add(TerminalSessionAttach.serializer)
          ..add(TerminalSessionChangedEvent.serializer)
          ..add(TerminalSessionChangedEventTypeEnum.serializer)
          ..add(TerminalSessionGrant.serializer)
          ..add(TerminalSessionGrantNoiseProtocolEnum.serializer)
          ..add(TerminalSessionGrantPrologueVersionEnum.serializer)
          ..add(TerminalSessionOpen.serializer)
          ..add(TerminalSessionPage.serializer)
          ..add(TerminalSessionState.serializer)
          ..add(TerminalStreamHello.serializer)
          ..add(TerminalStreamIdentify.serializer)
          ..add(Thread.serializer)
          ..add(ThreadClosedEvent.serializer)
          ..add(ThreadClosedEventTypeEnum.serializer)
          ..add(ThreadCreate.serializer)
          ..add(ThreadCreated.serializer)
          ..add(ThreadCreatedEvent.serializer)
          ..add(ThreadCreatedEventTypeEnum.serializer)
          ..add(ThreadKindEnum.serializer)
          ..add(ThreadMove.serializer)
          ..add(ThreadMovedEvent.serializer)
          ..add(ThreadMovedEventTypeEnum.serializer)
          ..add(ThreadMovedOutEvent.serializer)
          ..add(ThreadMovedOutEventTypeEnum.serializer)
          ..add(ThreadMovedOutPayload.serializer)
          ..add(ThreadMovedPayload.serializer)
          ..add(ThreadPage.serializer)
          ..add(ThreadPayload.serializer)
          ..add(ThreadReopenedEvent.serializer)
          ..add(ThreadReopenedEventTypeEnum.serializer)
          ..add(ThreadState.serializer)
          ..add(ThreadTagsPut.serializer)
          ..add(ThreadTitleHitList.serializer)
          ..add(ThreadTitleHitListItemsInner.serializer)
          ..add(ThreadTitlePut.serializer)
          ..add(ThreadTitleSourceEnum.serializer)
          ..add(ThreadUpdatedEvent.serializer)
          ..add(ThreadUpdatedEventTypeEnum.serializer)
          ..add(Thumbnail.serializer)
          ..add(ThumbnailVariantEnum.serializer)
          ..add(TokenCounts.serializer)
          ..add(TokenRefreshRequest.serializer)
          ..add(TokenResponse.serializer)
          ..add(TokenResponseTokenTypeEnum.serializer)
          ..add(ToolClass.serializer)
          ..add(ToolClassSet.serializer)
          ..add(Transcript.serializer)
          ..add(TranscriptSource.serializer)
          ..add(TranscriptStatus.serializer)
          ..add(TranscriptSubmit.serializer)
          ..add(TriggerState.serializer)
          ..add(TrustedHeaderLoginRequest.serializer)
          ..add(TypingStartedEvent.serializer)
          ..add(TypingStartedEventTypeEnum.serializer)
          ..add(TypingStartedPayload.serializer)
          ..add(UnreadThread.serializer)
          ..add(UploadComplete.serializer)
          ..add(UploadCreate.serializer)
          ..add(UploadPart.serializer)
          ..add(UploadPartResult.serializer)
          ..add(UploadSession.serializer)
          ..add(UploadSessionModeEnum.serializer)
          ..add(UploadState.serializer)
          ..add(UploadStateCompletedPartsInner.serializer)
          ..add(UploadStateUploadStateEnum.serializer)
          ..add(User.serializer)
          ..add(UserCreate.serializer)
          ..add(UserCreated.serializer)
          ..add(UserPage.serializer)
          ..add(UserPayload.serializer)
          ..add(UserSetting.serializer)
          ..add(UserSettingList.serializer)
          ..add(UserSettingPut.serializer)
          ..add(UserStatus.serializer)
          ..add(UserUpdate.serializer)
          ..add(UserUpdatedEvent.serializer)
          ..add(UserUpdatedEventTypeEnum.serializer)
          ..add(ViewAsSnapshot.serializer)
          ..add(WebAuthnChallenge.serializer)
          ..add(WebhookDelivery.serializer)
          ..add(WebhookDeliveryPage.serializer)
          ..add(WebhookDeliveryStatus.serializer)
          ..add(WebhookKey.serializer)
          ..add(WebhookKeyStateEnum.serializer)
          ..add(WebhookKeys.serializer)
          ..add(WebhookSubscription.serializer)
          ..add(WebhookSubscriptionCreate.serializer)
          ..add(WebhookSubscriptionCreated.serializer)
          ..add(WebhookSubscriptionList.serializer)
          ..add(WebhookSubscriptionSignatureSchemesEnum.serializer)
          ..add(WebhookSubscriptionStateEnum.serializer)
          ..add(WebhookSubscriptionUpdate.serializer)
          ..add(WebhookSubscriptionUpdateStateEnum.serializer)
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AccessEffect)]),
            () => ListBuilder<AccessEffect>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(InvariantViolation),
            ]),
            () => ListBuilder<InvariantViolation>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(PreviewHint)]),
            () => ListBuilder<PreviewHint>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AccessEntry)]),
            () => ListBuilder<AccessEntry>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AccessTarget)]),
            () => ListBuilder<AccessTarget>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AccessTarget)]),
            () => ListBuilder<AccessTarget>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AccessTarget)]),
            () => ListBuilder<AccessTarget>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Agent)]),
            () => ListBuilder<Agent>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(AgentChannelConfig),
            ]),
            () => ListBuilder<AgentChannelConfig>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AgentSession)]),
            () => ListBuilder<AgentSession>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ApiToken)]),
            () => ListBuilder<ApiToken>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(App)]),
            () => ListBuilder<App>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AppScope)]),
            () => ListBuilder<AppScope>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AppScope)]),
            () => ListBuilder<AppScope>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AppScope)]),
            () => ListBuilder<AppScope>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AppScope)]),
            () => ListBuilder<AppScope>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AppScope)]),
            () => ListBuilder<AppScope>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AppScope)]),
            () => ListBuilder<AppScope>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AppScope)]),
            () => ListBuilder<AppScope>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Attachment)]),
            () => ListBuilder<Attachment>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ReactionSummary)]),
            () => ListBuilder<ReactionSummary>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Component)]),
            () => ListBuilder<Component>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AuditEvent)]),
            () => ListBuilder<AuditEvent>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(BillingMode)]),
            () => ListBuilder<BillingMode>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatsBackendCost)]),
            () => ListBuilder<StatsBackendCost>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(BillingMode)]),
            () => ListBuilder<BillingMode>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(BillingMode)]),
            () => ListBuilder<BillingMode>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ProjectReadiness)]),
            () => ListBuilder<ProjectReadiness>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(BuiltMap, const [
                const FullType(String),
                const FullType.nullable(JsonObject),
              ]),
            ]),
            () => ListBuilder<BuiltMap<String, JsonObject?>>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Capability)]),
            () => ListBuilder<Capability>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Capability)]),
            () => ListBuilder<Capability>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Capability)]),
            () => ListBuilder<Capability>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Capability)]),
            () => ListBuilder<Capability>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Capability)]),
            () => ListBuilder<Capability>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Capability)]),
            () => ListBuilder<Capability>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Capability)]),
            () => ListBuilder<Capability>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Capability)]),
            () => ListBuilder<Capability>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AccessSource)]),
            () => ListBuilder<AccessSource>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ChannelView)]),
            () => ListBuilder<ChannelView>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ClientMetric)]),
            () => ListBuilder<ClientMetric>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(ClientMetricBucketsInner),
            ]),
            () => ListBuilder<ClientMetricBucketsInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Command)]),
            () => ListBuilder<Command>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CommandChoice)]),
            () => ListBuilder<CommandChoice>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CommandChoice)]),
            () => ListBuilder<CommandChoice>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CommandChoice)]),
            () => ListBuilder<CommandChoice>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CommandChoice)]),
            () => ListBuilder<CommandChoice>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CommandOption)]),
            () => ListBuilder<CommandOption>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CommandOption)]),
            () => ListBuilder<CommandOption>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(CommandOptionValue),
            ]),
            () => ListBuilder<CommandOptionValue>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(CommandOptionValue),
            ]),
            () => ListBuilder<CommandOptionValue>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(CommandOptionValue),
            ]),
            () => ListBuilder<CommandOptionValue>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(CommandOptionValue),
            ]),
            () => ListBuilder<CommandOptionValue>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(CommandOptionValue),
            ]),
            () => ListBuilder<CommandOptionValue>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(ConsoleSessionStepUpMethodsAvailableEnum),
            ]),
            () => ListBuilder<ConsoleSessionStepUpMethodsAvailableEnum>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CostBucket)]),
            () => ListBuilder<CostBucket>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CostEstimate)]),
            () => ListBuilder<CostEstimate>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CostReport)]),
            () => ListBuilder<CostReport>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CostTotal)]),
            () => ListBuilder<CostTotal>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(RunCost)]),
            () => ListBuilder<RunCost>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Device)]),
            () => ListBuilder<Device>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Draft)]),
            () => ListBuilder<Draft>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(EvaluatedFlag)]),
            () => ListBuilder<EvaluatedFlag>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Event)]),
            () => ListBuilder<Event>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(ExpiringAccessItem),
            ]),
            () => ListBuilder<ExpiringAccessItem>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(FeatureFlag)]),
            () => ListBuilder<FeatureFlag>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(FieldError)]),
            () => ListBuilder<FieldError>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(FieldError)]),
            () => ListBuilder<FieldError>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(InvariantViolation),
            ]),
            () => ListBuilder<InvariantViolation>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ForumTag)]),
            () => ListBuilder<ForumTag>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(GdprRequest)]),
            () => ListBuilder<GdprRequest>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Grant)]),
            () => ListBuilder<Grant>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Group)]),
            () => ListBuilder<Group>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(GroupMember)]),
            () => ListBuilder<GroupMember>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(HeaderPair)]),
            () => ListBuilder<HeaderPair>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Host)]),
            () => ListBuilder<Host>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(HostProfile)]),
            () => ListBuilder<HostProfile>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(HostDoctorCheck)]),
            () => ListBuilder<HostDoctorCheck>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(HostSessionUser)]),
            () => ListBuilder<HostSessionUser>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(HostReportedSessionUser),
            ]),
            () => ListBuilder<HostReportedSessionUser>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(HostProfile)]),
            () => ListBuilder<HostProfile>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(HostReportedSessionUser),
            ]),
            () => ListBuilder<HostReportedSessionUser>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(HostDoctorCheck)]),
            () => ListBuilder<HostDoctorCheck>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(HostSessionUserAssign),
            ]),
            () => ListBuilder<HostSessionUserAssign>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(HostSessionUserAssign),
            ]),
            () => ListBuilder<HostSessionUserAssign>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Identity)]),
            () => ListBuilder<Identity>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Passkey)]),
            () => ListBuilder<Passkey>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(IdentityProvider)]),
            () => ListBuilder<IdentityProvider>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ImportItem)]),
            () => ListBuilder<ImportItem>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ImportItem)]),
            () => ListBuilder<ImportItem>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ImportObject)]),
            () => ListBuilder<ImportObject>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ImportSource)]),
            () => ListBuilder<ImportSource>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(ImportSourceDiscordConfigSpacesInner),
            ]),
            () => ListBuilder<ImportSourceDiscordConfigSpacesInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(ImportSourceDiscordConfigChannelsInner),
            ]),
            () => ListBuilder<ImportSourceDiscordConfigChannelsInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(ImportSourceDiscordConfigUsersInner),
            ]),
            () => ListBuilder<ImportSourceDiscordConfigUsersInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(IncomingWebhook)]),
            () => ListBuilder<IncomingWebhook>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(InstanceSetting)]),
            () => ListBuilder<InstanceSetting>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(JsonPatchOperation),
            ]),
            () => ListBuilder<JsonPatchOperation>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(JsonPatchOperation),
            ]),
            () => ListBuilder<JsonPatchOperation>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(JsonPatchOperation),
            ]),
            () => ListBuilder<JsonPatchOperation>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(MentionItem)]),
            () => ListBuilder<MentionItem>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Message)]),
            () => ListBuilder<Message>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(MessageRevision)]),
            () => ListBuilder<MessageRevision>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Money)]),
            () => ListBuilder<Money>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(NotificationPreference),
            ]),
            () => ListBuilder<NotificationPreference>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Owner)]),
            () => ListBuilder<Owner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(PermissionHint)]),
            () => ListBuilder<PermissionHint>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(PermissionHintRulesEnum),
            ]),
            () => ListBuilder<PermissionHintRulesEnum>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Pin)]),
            () => ListBuilder<Pin>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Price)]),
            () => ListBuilder<Price>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(PrincipalRef)]),
            () => ListBuilder<PrincipalRef>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AccessLabel)]),
            () => ListBuilder<AccessLabel>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AccessLabel)]),
            () => ListBuilder<AccessLabel>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ChannelLabel)]),
            () => ListBuilder<ChannelLabel>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(PushRegistration)]),
            () => ListBuilder<PushRegistration>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(QuietHoursRule)]),
            () => ListBuilder<QuietHoursRule>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(QuietHoursRule)]),
            () => ListBuilder<QuietHoursRule>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Reaction)]),
            () => ListBuilder<Reaction>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ReadState)]),
            () => ListBuilder<ReadState>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ReconcileEntry)]),
            () => ListBuilder<ReconcileEntry>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(RunContextAttachment),
            ]),
            () => ListBuilder<RunContextAttachment>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(RunContextMessage),
            ]),
            () => ListBuilder<RunContextMessage>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(RunContextMessage),
            ]),
            () => ListBuilder<RunContextMessage>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(RunPlanItemsInner),
            ]),
            () => ListBuilder<RunPlanItemsInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(RunStep)]),
            () => ListBuilder<RunStep>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(RunSummary)]),
            () => ListBuilder<RunSummary>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(SearchHit)]),
            () => ListBuilder<SearchHit>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(SearchHitSegmentsInner),
            ]),
            () => ListBuilder<SearchHitSegmentsInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Session)]),
            () => ListBuilder<Session>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(SidebarState)]),
            () => ListBuilder<SidebarState>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(SpaceView)]),
            () => ListBuilder<SpaceView>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(SpaceView)]),
            () => ListBuilder<SpaceView>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ChannelView)]),
            () => ListBuilder<ChannelView>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(SpaceView)]),
            () => ListBuilder<SpaceView>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ChannelView)]),
            () => ListBuilder<ChannelView>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(SidebarState)]),
            () => ListBuilder<SidebarState>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(UnreadThread)]),
            () => ListBuilder<UnreadThread>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(RunSummary)]),
            () => ListBuilder<RunSummary>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Thread)]),
            () => ListBuilder<Thread>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AgentPresence)]),
            () => ListBuilder<AgentPresence>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(UserSetting)]),
            () => ListBuilder<UserSetting>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(DraftRef)]),
            () => ListBuilder<DraftRef>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(EvaluatedFlag)]),
            () => ListBuilder<EvaluatedFlag>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(StatsBackendAgent),
            ]),
            () => ListBuilder<StatsBackendAgent>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatsBucket)]),
            () => ListBuilder<StatsBucket>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatsLiveRun)]),
            () => ListBuilder<StatsLiveRun>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatsLiveAgent)]),
            () => ListBuilder<StatsLiveAgent>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatsLiveRate)]),
            () => ListBuilder<StatsLiveRate>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatsLiveSli)]),
            () => ListBuilder<StatsLiveSli>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatsSloEntry)]),
            () => ListBuilder<StatsSloEntry>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(StatsTimelineBucket),
            ]),
            () => ListBuilder<StatsTimelineBucket>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatsTimelineRun)]),
            () => ListBuilder<StatsTimelineRun>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatusComponent)]),
            () => ListBuilder<StatusComponent>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(StatusIncident)]),
            () => ListBuilder<StatusIncident>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(CommandRegister)]),
            () => ListBuilder<CommandRegister>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(AppManifestComponentsEnum),
            ]),
            () => ListBuilder<AppManifestComponentsEnum>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(AppManifestRequestedGrantsInner),
            ]),
            () => ListBuilder<AppManifestRequestedGrantsInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(AppManifestIncomingWebhooksInner),
            ]),
            () => ListBuilder<AppManifestIncomingWebhooksInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(StatusIncidentUpdate),
            ]),
            () => ListBuilder<StatusIncidentUpdate>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(AuthProviderInfo)]),
            () => ListBuilder<AuthProviderInfo>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(ClientVersionRequirement),
            ]),
            () => ListBuilder<ClientVersionRequirement>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(ComponentCreate)]),
            () => ListBuilder<ComponentCreate>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(WebhookSubscriptionSignatureSchemesEnum),
            ]),
            () => ListBuilder<WebhookSubscriptionSignatureSchemesEnum>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Template)]),
            () => ListBuilder<Template>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(TerminalSession)]),
            () => ListBuilder<TerminalSession>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Thread)]),
            () => ListBuilder<Thread>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(ThreadTitleHitListItemsInner),
            ]),
            () => ListBuilder<ThreadTitleHitListItemsInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(Thumbnail)]),
            () => ListBuilder<Thumbnail>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(UploadPart)]),
            () => ListBuilder<UploadPart>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(UploadPartResult)]),
            () => ListBuilder<UploadPartResult>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(UploadStateCompletedPartsInner),
            ]),
            () => ListBuilder<UploadStateCompletedPartsInner>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(User)]),
            () => ListBuilder<User>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(UserSetting)]),
            () => ListBuilder<UserSetting>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(WebhookDelivery)]),
            () => ListBuilder<WebhookDelivery>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(WebhookKey)]),
            () => ListBuilder<WebhookKey>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(WebhookSubscription),
            ]),
            () => ListBuilder<WebhookSubscription>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(int)]),
            () => ListBuilder<int>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(int)]),
            () => ListBuilder<int>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(int)]),
            () => ListBuilder<int>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(int)]),
            () => ListBuilder<int>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType(RunStep),
            ]),
            () => MapBuilder<String, RunStep>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType(RunFollowup),
            ]),
            () => MapBuilder<String, RunFollowup>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType(double),
            ]),
            () => MapBuilder<String, double>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(String)]),
            () => ListBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [const FullType(HostProfile)]),
            () => ListBuilder<HostProfile>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(HostReportedSessionUser),
            ]),
            () => ListBuilder<HostReportedSessionUser>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltMap, const [
              const FullType(String),
              const FullType.nullable(JsonObject),
            ]),
            () => MapBuilder<String, JsonObject?>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltList, const [
              const FullType(AlertmanagerAlert),
            ]),
            () => ListBuilder<AlertmanagerAlert>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltSet, const [const FullType(String)]),
            () => SetBuilder<String>(),
          )
          ..addBuilderFactory(
            const FullType(BuiltSet, const [const FullType(String)]),
            () => SetBuilder<String>(),
          ))
        .build();

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
