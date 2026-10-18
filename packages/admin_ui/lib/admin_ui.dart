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
library;

export 'src/access/source_chain.dart';
export 'src/access/view_as_person.dart';
export 'src/access/who_has_access.dart';
export 'src/common/admin_action.dart';
export 'src/common/code_text.dart';
export 'src/common/confirm_change.dart';
export 'src/common/problem_banner.dart';
export 'src/data/access_repository.dart';
export 'src/data/admin_repository.dart';
export 'src/data/self_repository.dart';
export 'src/devices/approve_device_dialog.dart';
export 'src/devices/waiting_devices.dart';
export 'src/grants/grant_wizard.dart';
export 'src/grants/grants_list.dart';
export 'src/grants/preview_panel.dart';
export 'src/grants/revoke_grant_dialog.dart';
export 'src/groups/group_detail.dart';
export 'src/groups/groups_list.dart';
export 'src/groups/member_editor.dart';
export 'src/people/create_person_dialog.dart';
export 'src/people/credentials_section.dart';
export 'src/people/enrollment_code_view.dart';
export 'src/people/owners_section.dart';
export 'src/people/people_list.dart';
export 'src/people/person_detail.dart';
export 'src/self/add_device_dialog.dart';
export 'src/self/confirm_device_dialog.dart';
export 'src/self/device_tile.dart';
export 'src/self/my_devices_view.dart';
export 'src/self/my_sessions_view.dart';
export 'src/self/revoke_confirm.dart';
export 'src/self/self_feedback.dart' show SelfFailure, SelfNotice;
export 'src/shell/admin_routes.dart';
export 'src/shell/admin_shell.dart';
export 'src/step_up/step_up_dialog.dart';
