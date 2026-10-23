// SPDX-License-Identifier: Apache-2.0

/// The plain texts of the apps screens: the meaning of each scope of 05 2.6, the effects of
/// 11 4.3, the problems of a manifest file the client could not read and the states of an
/// installation (11 4.1: the owner reads in plain text what an app may do; brief M6-08).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import '../data/apps_repository.dart';
import '../grants/grant_labels.dart' show levelWithCapabilities;

/// The meaning of the scopes of 05 2.6 by wire name. The table there is the only source of
/// which actions a scope allows; these texts say it in plain words for the consent.
const Map<String, String Function(AppLocalizations l)> _scopeMeanings = {
  'messages:read': _messagesRead,
  'messages:write': _messagesWrite,
  'files:read': _filesRead,
  'files:write': _filesWrite,
  'commands:write': _commandsWrite,
  'components:write': _componentsWrite,
  'users:read': _usersRead,
  'events:subscribe': _eventsSubscribe,
  'costs:reconcile': _costsReconcile,
};

String _messagesRead(AppLocalizations l) => l.adminAppScopeMessagesRead;
String _messagesWrite(AppLocalizations l) => l.adminAppScopeMessagesWrite;
String _filesRead(AppLocalizations l) => l.adminAppScopeFilesRead;
String _filesWrite(AppLocalizations l) => l.adminAppScopeFilesWrite;
String _commandsWrite(AppLocalizations l) => l.adminAppScopeCommandsWrite;
String _componentsWrite(AppLocalizations l) => l.adminAppScopeComponentsWrite;
String _usersRead(AppLocalizations l) => l.adminAppScopeUsersRead;
String _eventsSubscribe(AppLocalizations l) => l.adminAppScopeEventsSubscribe;
String _costsReconcile(AppLocalizations l) => l.adminAppScopeCostsReconcile;

/// Whether this version knows the meaning of [scope] (a wire name such as `messages:read`).
bool appScopeKnown(String scope) => _scopeMeanings.containsKey(scope);

/// What [scope] lets an app do, in plain words; a scope of a newer server reads "No
/// description in this version" (the consent still shows its wire name next to it).
///
/// Example: `appScopeLabel(l, 'files:write')` is "Upload files".
String appScopeLabel(AppLocalizations l, String scope) =>
    _scopeMeanings[scope]?.call(l) ?? l.adminAppScopeUnknown;

/// The text of a manifest file the client could not read ([ManifestFileError]).
String manifestFileProblemText(
  AppLocalizations l,
  ManifestFileProblem problem,
) => switch (problem) {
  ManifestFileProblem.notText => l.adminAppFileNotText,
  ManifestFileProblem.notJsonOrYaml => l.adminAppFileNotJsonOrYaml,
  ManifestFileProblem.notObject => l.adminAppFileNotObject,
};

/// The label of the installation state [state].
String appStateLabel(AppLocalizations l, AppInstallState state) =>
    switch (state) {
      AppInstallState.active => l.adminAppStateActive,
      AppInstallState.suspended => l.adminAppStateSuspended,
      AppInstallState.uninstalled => l.adminAppStateUninstalled,
      AppInstallState.unknown => l.adminAppStateUnknown,
    };

/// The plain name of the component type [type] of a manifest (`button`, `select`, `form`),
/// or the type itself for one this version does not know.
String appComponentLabel(AppLocalizations l, String type) => switch (type) {
  'button' => l.adminAppComponentButton,
  'select' => l.adminAppComponentSelect,
  'form' => l.adminAppComponentForm,
  _ => type,
};

/// A requested grant in plain words without its target, for example "Write on one channel"
/// (the owner picks the channel, 11 4.1).
String appGrantAskLabel(AppLocalizations l, AppGrantAsk ask) {
  final level = levelWithCapabilities(l, ask.level, ask.capabilities.toList());
  return switch (ask.targetKind) {
    AdminTargetKind.channel => l.adminAppGrantAskChannel(level),
    AdminTargetKind.space => l.adminAppGrantAskSpace(level),
    AdminTargetKind.instance => l.adminAppGrantAskInstance(level),
    _ => l.adminAppGrantAskOther(level),
  };
}

/// The badge of an installation state: active plain, suspended as a warning, uninstalled and
/// unknown as information.
///
/// Example: `AppStateBadge(state: app.state, key: const ValueKey('appPage.state'))`.
class AppStateBadge extends StatelessWidget {
  /// Creates the badge of [state].
  const AppStateBadge({super.key, required this.state});

  /// The state.
  final AppInstallState state;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final ({Color background, Color foreground}) look = switch (state) {
      AppInstallState.active => (
        background: colors.accentSubtle,
        foreground: colors.textPrimary,
      ),
      AppInstallState.suspended => () {
        final t = dsToneColors(context, DsTone.warning);
        return (background: t.subtle, foreground: t.fg);
      }(),
      _ => () {
        final t = dsToneColors(context, DsTone.info);
        return (background: t.subtle, foreground: t.fg);
      }(),
    };
    return DsBadge(
      text: appStateLabel(l, state),
      background: look.background,
      foreground: look.foreground,
    );
  }
}

/// The lifecycle actions of 11 4.3 whose dialog shows the table of effects.
enum AppLifecycleAction {
  /// `app.suspend`.
  suspend,

  /// `app.resume`: the table says what comes back.
  resume,

  /// `app.uninstall`.
  uninstall,
}

/// One row of the table of effects: the `part` that is affected and the `effect` on it.
typedef AppEffectRow = ({String part, String effect});

/// The column of the table of 11 4.3 for [action], in the rows of the table: tokens,
/// deliveries, commands, components, grants and incoming webhooks, messages. For
/// [AppLifecycleAction.resume] the column says what comes back.
List<AppEffectRow> lifecycleEffects(
  AppLocalizations l,
  AppLifecycleAction action,
) {
  final effects = switch (action) {
    AppLifecycleAction.suspend => [
      l.adminAppEffectBlocked,
      l.adminAppEffectPaused,
      l.adminAppEffectHidden,
      l.adminAppEffectDisabled,
      l.adminAppEffectSuspendGrants,
      l.adminAppEffectRemain,
    ],
    AppLifecycleAction.uninstall => [
      l.adminAppEffectRevoked,
      l.adminAppEffectSubscriptionsDisabled,
      l.adminAppEffectRemoved,
      l.adminAppEffectDisabled,
      l.adminAppEffectRevoked,
      l.adminAppEffectUninstallMessages,
    ],
    AppLifecycleAction.resume => [
      l.adminAppEffectWorkAgain,
      l.adminAppEffectDeliveriesContinue,
      l.adminAppEffectShownAgain,
      l.adminAppEffectEnabledAgain,
      l.adminAppEffectResumeGrants,
      l.adminAppEffectUnchanged,
    ],
  };
  final parts = [
    l.adminAppEffectRowTokens,
    l.adminAppEffectRowDeliveries,
    l.adminAppEffectRowCommands,
    l.adminAppEffectRowComponents,
    l.adminAppEffectRowGrants,
    l.adminAppEffectRowMessages,
  ];
  return [
    for (final (i, part) in parts.indexed) (part: part, effect: effects[i]),
  ];
}
