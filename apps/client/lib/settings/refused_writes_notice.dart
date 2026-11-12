// SPDX-License-Identifier: Apache-2.0

/// The notice of the settings for writes the instance refused for good
/// (docs/spec/04-sync-protocol.md 13.2 item 5: "The entry stays visible as failed with a
/// reason; the person can send it again or discard it";
/// docs/decisions.d/AUD-client-settings-01.md).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import 'settings_providers.dart';

/// The label of the setting [key] as its section shows it, or the key itself for a setting
/// that has no row in D8 (a key of the notification settings, a key a newer app wrote).
String settingLabel(AppLocalizations l, String key) => switch (key) {
  SettingKeys.theme => l.settingsTheme,
  SettingKeys.density => l.settingsDensity,
  SettingKeys.reduceMotion => l.settingsMotion,
  SettingKeys.enterSends => l.settingsEnterSends,
  SettingKeys.pasteOriginals => l.settingsPasteOriginals,
  SettingKeys.crashReports => l.settingsCrashReports,
  SettingKeys.clientMetrics => l.settingsClientMetrics,
  SettingKeys.columnWidths => l.settingsColumnWidths,
  SettingKeys.autoArrangeSpaces => l.settingsAutoArrangeSpaces,
  SettingKeys.autoArrangeChannels => l.settingsAutoArrangeChannels,
  SettingKeys.autoArrangeChats => l.settingsAutoArrangeChats,
  SettingKeys.showProjectStatus => l.settingsShowProjectStatus,
  _ => key,
};

/// One banner per refused write of a personal setting, each with its reason, "Send again"
/// and "Discard"; nothing while there is none. It stands above every section of the
/// settings, because the write may come from any of them and from outside the settings (the
/// column widths, the notification settings).
class RefusedSettingWritesNotice extends ConsumerStatefulWidget {
  /// Creates the notice.
  const RefusedSettingWritesNotice({super.key});

  @override
  ConsumerState<RefusedSettingWritesNotice> createState() =>
      _RefusedSettingWritesNoticeState();
}

class _RefusedSettingWritesNoticeState
    extends ConsumerState<RefusedSettingWritesNotice> {
  /// The entries an action runs for.
  final Set<String> _busy = {};

  /// The text of a failed discard, by entry.
  final Map<String, String> _discardFailed = {};

  Future<void> _run(
    RefusedSettingWrite write,
    Future<void> Function(RefusedSettingActions actions) action,
  ) async {
    final actions = ref.read(refusedSettingActionsProvider);
    if (actions == null || _busy.contains(write.entryId)) return;
    final l = AppLocalizations.of(context);
    setState(() {
      _busy.add(write.entryId);
      _discardFailed.remove(write.entryId);
    });
    String? failed;
    try {
      await action(actions);
    } on DioException catch (e) {
      final problem = ApiProblem.of(e);
      failed = l.settingsWriteDiscardFailed(
        problem == null
            ? l.selfOffline
            : problemText(l, problem.code, reason: problem.reason),
      );
    }
    if (!mounted) return;
    setState(() {
      _busy.remove(write.entryId);
      if (failed != null) _discardFailed[write.entryId] = failed;
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final writes =
        ref.watch(refusedSettingWritesProvider).value ??
        const <RefusedSettingWrite>[];
    if (writes.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final write in writes)
          Padding(
            key: const ValueKey('settings.refusedWrite'),
            padding: const EdgeInsets.only(bottom: DsSpace.s16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                DsBanner(
                  tone: DsTone.danger,
                  text: l.settingsWriteRefused(
                    settingLabel(l, write.key),
                    problemText(l, write.code, reason: write.reason),
                  ),
                  liveRegion: true,
                ),
                if (_discardFailed[write.entryId] case final text?)
                  Padding(
                    padding: const EdgeInsets.only(top: DsSpace.s4),
                    child: Text(
                      text,
                      style: dsTextStyle(DsType.bodySm)
                          .copyWith(color: colors.dangerFg),
                    ),
                  ),
                Wrap(
                  spacing: DsSpace.s8,
                  children: [
                    TextButton(
                      onPressed: _busy.contains(write.entryId)
                          ? null
                          : () => _run(write, (a) => a.sendAgain(write)),
                      child: Text(l.settingsWriteSendAgain),
                    ),
                    TextButton(
                      onPressed: _busy.contains(write.entryId)
                          ? null
                          : () => _run(write, (a) => a.discard(write)),
                      child: Text(l.settingsWriteDiscard),
                    ),
                  ],
                ),
              ],
            ),
          ),
      ],
    );
  }
}
