// SPDX-License-Identifier: Apache-2.0

/// Administration screens for the desktop app, the console and the emergency actions on mobile
/// (docs/spec/15-client-architecture.md 3, 3.1; R-PR-07).
///
/// M4-25 adds the person's own devices and sessions: [MyDevicesView] (confirm a waiting device
/// with its code, add a device by its pairing code, revoke a device), [MySessionsView] (end a
/// session), the [StepUpDialog] with its [StepUpDialogPrompt] (the `StepUpPrompt` of
/// `package:auth`) and the REST data source [ApiSelfRepository]. The package uses REST only
/// and no local database, so the console can use the same screens.
///
/// M4-26 adds the administration of A1 (01 5.2): the [AdminShell] with its places
/// ([AdminSection], [AdminLocation]), the people ([PeopleList], [PersonDetail],
/// [CreatePersonDialog], [EnrollmentCodeView], [CredentialsSection], [OwnersSection]), the
/// groups ([GroupsList], [GroupDetail], [MemberEditor]), the queue of waiting devices
/// ([WaitingDevicesView], [ApproveDeviceDialog]), the confirmation before a change
/// ([ConfirmChange]), the problem texts ([ProblemBanner], [adminFailure]), the code view
/// ([CodeText]) and the REST data source [ApiAdminRepository].
///
/// M4-27 adds the permission screens A2 to A4 (01 5.2; 07 9): the [GrantWizard] in three
/// steps with the server's [PreviewPanel], the [GrantsList] per target with
/// [RevokeGrantDialog], [WhoHasAccess] with its [SourceChain], [ViewAsPerson] and the data
/// source [AccessRepository] ([ApiAccessRepository]), which [AdminRepository] includes.
///
/// M4-29 adds the audit log of A6: [AuditLogView] with its [AuditFilters], the details
/// ([AuditEntryDetail]), "Check chain" ([AuditChainCheckButton], [AuditChainPanel]), the export
/// ([AuditExportBar], [AuditSaveTarget], [auditNdjsonToCsv]) and the data source
/// [AuditRepository] ([ApiAuditRepository]), which [AdminRepository] includes.
///
/// M4-37 adds screen A8 for owners (01 5.2; 24 2.2, 3, 7, 8): the [HostsList], the
/// [HostDetail] with [LlmLoginSection], [ServerHostSection], [DoctorResults],
/// [HostGrantsSection], [SessionUsersSection] and [HostStateActions], the [PairingWizard] with
/// [CreateHostDialog], [FingerprintConfirm], [LlmLoginChoice], [ServerHostChoice] and
/// [SessionUserRows], the host target in the [GrantWizard] and the data source
/// [HostsRepository] ([ApiHostsRepository]), which [AdminRepository] includes.
///
/// M4-28 adds screen A5 (01 5.2; 10 5.4, 8, 9, 11, 13): the [AgentsList] with the
/// [AgentModeBadge], the [AgentPage] with the configured next to the reported mode and the
/// server's warnings, [SwitchModeDialog], [HolderDialog], [SwitchAllDialog], the
/// [I1Explanation], the agent's [AgentTokens] with [TokenSecretView], the guided flow
/// ([GuidedSwitch], [GuidedFlow], [GuidedResult]) in the grant preview, the group members and
/// the owners, and the data source [AgentsRepository] ([ApiAgentsRepository]), which
/// [AdminRepository] includes.
///
/// M6-04 adds what the console needs to host the administration (01 5.2, 5.5): the
/// [AdminBody] (the screen of one place without the navigation, at the full browser width when
/// its `maxWidth` is null, which [AdminShell] uses on the desktop), the hook [AdminViewAs] that
/// [WhoHasAccess], [PersonDetail] and [AdminBody] hand the chosen person to, and the stored
/// view of "View as person": the [ViewAsBanner] with the time left and "End view", the
/// [ViewAsStructure] (structure and access only, never content; ADR 0028) and
/// [notVisibleSpaces].
///
/// M6-08 adds the apps and webhooks of 01 5.2 and 11 4 to 8: the [AppsList] with the install
/// flow from a manifest file ([ManifestPicker], [ManifestUpload]) and its consent, the
/// [AppPage] with the update diff, suspend, resume and uninstall and the outgoing
/// [AppSubscriptionsSection] with its deliveries, the [IncomingWebhooksView] per channel with
/// the composed "incoming app", the shared [OnceOnlySecret] for every secret shown once, and
/// the data source [AppsRepository] ([ApiAppsRepository]), which [AdminRepository] includes.
///
/// M6-07 adds the lists of 07 9.4 and 9.5 as the places [AdminSection.hints] and
/// [AdminSection.expiring]: [PermissionHintsView] (H1, grants to disabled principals, blocked
/// `trigger` grants, agents in mode `personal`; each hint as a sentence from the templates of
/// `l10n`, see [hintLines]) and [ExpiringAccessView] (grants, memberships and tokens that
/// expire soon, with "Extend" as a new grant and then the revocation of the old one), and
/// their data in [AccessRepository] ([AccessHintPage], [ExpiringPage], [AccessPageLabels]).
///
/// M6-09 adds the place [AdminSection.instance] for owners (01 5.5 "Instance, branding"): the
/// [InstanceView] with the [InstanceSettingsSection] (typed editors per key of the configuration
/// reference section 6, [instanceSettingSchemas], [SettingEditorDialog]), the
/// [BrandingSection] (product name, logo through a [LogoPicker], [AccentPicker] of the seven
/// palette keys with its [AccentPreview] in light and dark), the [ProvidersSection] with the
/// [ProviderEditorDialog] (OIDC and proxy header per 08 2.6, write-only secret references, the
/// [RedirectUriView]) and the [FlagsSection] with its [FlagSwitchDialog], and the data source
/// [InstanceRepository] ([ApiInstanceRepository]), which [AdminRepository] includes. A write
/// that loses against a concurrent change ends in [InstanceVersionConflict].
///
/// M8-08 adds the import of 01 3.7 (web only, [AdminSection.dataImport]): the [ImportListPage]
/// with "Add source" ([ImportCreateDialog], the mapping file converted by [mappingJsonOf], the
/// server's pointers translated by [mappingPathOf]), the [ImportSourcePage] with the counters,
/// the failed items, the state steps ([ImportStateAction], [ImportConfirm]) and the
/// [ImportChannelTable] with the lead steps and the [ImportOperatorDialog], the
/// [ImportController] over the [ImportSourceSnapshot] that `getImport` and `import.progress`
/// carry, and the data source [ImportRepository] ([ApiImportRepository]), which
/// [AdminRepository] includes.
///
/// M5-09 adds the emergency actions on the phone (01 3.7; R-PR-07): the [EmergencyHome] with
/// the person search, the [PersonEmergencyView] (revoke a device, revoke a grant, disable the
/// person) and the [EmergencyConfirmSheet] that says the action takes effect at once and
/// names the step-up ([emergencyStepUpLabel]); failures read through [emergencyFailure].
/// They are phone layouts over [AdminRepository] and [AdminActionState]; the grants of one
/// person come from [AccessRepository.grantsOf].
library;

export 'src/access/source_chain.dart';
export 'src/access/view_as_banner.dart';
export 'src/access/view_as_person.dart';
export 'src/access/view_as_structure.dart';
export 'src/access/who_has_access.dart';
export 'src/agents/agent_labels.dart';
export 'src/agents/agent_page.dart';
export 'src/agents/agent_tokens.dart';
export 'src/agents/agents_list.dart';
export 'src/agents/guided_flow.dart';
export 'src/agents/holder_dialog.dart';
export 'src/agents/i1_explanation.dart';
export 'src/agents/switch_all_dialog.dart';
export 'src/agents/switch_mode_dialog.dart';
export 'src/apps/app_labels.dart';
export 'src/apps/app_page.dart';
export 'src/apps/apps_list.dart';
export 'src/apps/consent_view.dart';
export 'src/apps/deliveries_view.dart';
export 'src/apps/install_dialog.dart';
export 'src/apps/lifecycle_dialog.dart';
export 'src/apps/manifest_picker.dart';
export 'src/apps/subscriptions_section.dart';
export 'src/apps/update_dialog.dart';
export 'src/apps/webhook_keys_card.dart';
export 'src/audit/audit_entry_detail.dart';
export 'src/audit/audit_export.dart';
export 'src/audit/audit_filters.dart';
export 'src/audit/audit_list.dart';
export 'src/audit/audit_repository.dart';
export 'src/audit/chain_check.dart';
export 'src/common/admin_action.dart';
export 'src/common/code_text.dart';
export 'src/common/confirm_change.dart';
export 'src/common/principal_tokens.dart';
export 'src/common/problem_banner.dart';
export 'src/data/access_repository.dart';
export 'src/data/admin_repository.dart';
export 'src/data/agents_repository.dart';
export 'src/data/apps_repository.dart';
export 'src/data/hosts_repository.dart';
export 'src/data/instance_repository.dart';
export 'src/data/self_repository.dart';
export 'src/devices/approve_device_dialog.dart';
export 'src/devices/waiting_devices.dart';
export 'src/emergency/emergency_home.dart';
export 'src/emergency/emergency_sheet.dart';
export 'src/emergency/person_emergency_view.dart';
export 'src/grants/grant_labels.dart' show AccessNames;
export 'src/grants/grant_wizard.dart';
export 'src/grants/grants_list.dart';
export 'src/grants/preview_panel.dart';
export 'src/grants/revoke_grant_dialog.dart';
export 'src/groups/group_detail.dart';
export 'src/groups/groups_list.dart';
export 'src/groups/member_editor.dart';
export 'src/hints/access_list.dart' show AccessListView;
export 'src/hints/expiring_access_view.dart';
export 'src/hints/hint_texts.dart';
export 'src/hints/permission_hints_view.dart';
export 'src/hosts/create_host_dialog.dart';
export 'src/hosts/doctor_results.dart';
export 'src/hosts/fingerprint_confirm.dart';
export 'src/hosts/host_grants_section.dart';
export 'src/hosts/host_labels.dart';
export 'src/hosts/host_page.dart';
export 'src/hosts/host_state_actions.dart';
export 'src/hosts/hosts_list.dart';
export 'src/hosts/llm_login_section.dart';
export 'src/hosts/pairing_wizard.dart';
export 'src/hosts/server_host_section.dart';
export 'src/hosts/session_users_section.dart';
export 'src/import/import_channel_table.dart';
export 'src/import/import_controller.dart';
export 'src/import/import_create_dialog.dart';
export 'src/import/import_dialogs.dart';
export 'src/import/import_labels.dart';
export 'src/import/import_list_page.dart';
export 'src/import/import_repository.dart';
export 'src/import/import_source_page.dart';
export 'src/import/mapping_file.dart';
export 'src/incoming_webhooks/create_incoming_dialog.dart';
export 'src/incoming_webhooks/incoming_webhooks_view.dart';
export 'src/instance/accent_picker.dart';
export 'src/instance/branding_section.dart';
export 'src/instance/flags_section.dart';
export 'src/instance/instance_common.dart';
export 'src/instance/instance_view.dart';
export 'src/instance/provider_editor.dart';
export 'src/instance/providers_section.dart';
export 'src/instance/setting_kinds.dart';
export 'src/instance/settings_section.dart';
export 'src/people/create_person_dialog.dart';
export 'src/people/credentials_section.dart';
export 'src/people/enrollment_code_view.dart';
export 'src/people/owners_section.dart';
export 'src/people/people_list.dart';
export 'src/people/person_detail.dart';
export 'src/secrets/once_only_secret.dart';
export 'src/self/add_device_dialog.dart';
export 'src/self/confirm_device_dialog.dart';
export 'src/self/device_tile.dart';
export 'src/self/my_devices_view.dart';
export 'src/self/my_sessions_view.dart';
export 'src/self/revoke_confirm.dart';
export 'src/self/self_feedback.dart' show SelfFailure, SelfNotice;
export 'src/shell/admin_body.dart';
export 'src/shell/admin_routes.dart';
export 'src/shell/admin_shell.dart';
export 'src/stats/stats_controller.dart';
export 'src/stats/stats_range.dart';
export 'src/stats/stats_screen.dart';
export 'src/stats/stats_sections.dart';
export 'src/stats/stats_source.dart';
export 'src/step_up/step_up_dialog.dart';
