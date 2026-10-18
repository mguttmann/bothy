// SPDX-License-Identifier: Apache-2.0

/// Administration screens for the desktop app, the console and the emergency actions on mobile
/// (docs/spec/15-client-architecture.md 3, 3.1; R-PR-07).
///
/// M4-25 adds the person's own devices and sessions: [MyDevicesView] (confirm a waiting device
/// with its code, add a device by its pairing code, revoke a device), [MySessionsView] (end a
/// session), the [StepUpDialog] with its [StepUpDialogPrompt] (the `StepUpPrompt` of
/// `package:auth`) and the REST data source [ApiSelfRepository]. The package uses REST only
/// and no local database, so the console can use the same screens.
library;

export 'src/data/self_repository.dart';
export 'src/self/add_device_dialog.dart';
export 'src/self/confirm_device_dialog.dart';
export 'src/self/device_tile.dart';
export 'src/self/my_devices_view.dart';
export 'src/self/my_sessions_view.dart';
export 'src/self/revoke_confirm.dart';
export 'src/self/self_feedback.dart' show SelfNotice;
export 'src/step_up/step_up_dialog.dart';
