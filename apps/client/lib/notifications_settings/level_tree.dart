// SPDX-License-Identifier: Apache-2.0

/// The tree "Per space, channel and thread" of D9 (01 5.1 D9; 13 3.1, 3.2; brief M4-33 Scope):
/// each space, its channels and the threads with an own level, each with its level control.
/// A target without an own level shows what it inherits ("inherits: All"), a followed thread
/// "Following", an active mute its end.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../menus/level_menu.dart';
import '../settings/settings_providers.dart';
import 'effective_level.dart';
import 'providers.dart';

/// The value of the "Inherit" choice of a level control.
const String inheritChoice = 'inherit';

/// The tree of D9.
class LevelTree extends ConsumerWidget {
  /// Creates the tree.
  const LevelTree({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final tree = ref.watch(levelTreeProvider).value ?? const [];
    final levels = ref.watch(levelContextProvider);
    if (tree.isEmpty) {
      return Text(
        l.notifySettingsTreeEmpty,
        style: dsTextStyle(DsType.body)
            .copyWith(color: context.dsColors.textSecondary),
      );
    }
    return Column(
      key: const ValueKey('notify.tree'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final s in tree) ...[
          LevelTreeRow(
            key: ValueKey('tree.space.${s.space.id.value}'),
            depth: 0,
            name: s.space.name,
            target: LevelMenuTarget(
              kind: NotificationTargetKind.space,
              id: s.space.id.value,
              own: levels.own(NotificationTargetKind.space, s.space.id.value),
              inherited: levels.inherited(NotificationTargetKind.space),
              now: levels.now,
            ),
          ),
          for (final c in s.channels) ...[
            LevelTreeRow(
              key: ValueKey('tree.channel.${c.channel.id.value}'),
              depth: 1,
              name: c.channel.kind == ChannelKind.forum
                  ? '= ${c.channel.name}'
                  : '# ${c.channel.name}',
              target: LevelMenuTarget(
                kind: NotificationTargetKind.channel,
                id: c.channel.id.value,
                own: levels.own(
                  NotificationTargetKind.channel,
                  c.channel.id.value,
                ),
                inherited: levels.inherited(
                  NotificationTargetKind.channel,
                  channel: c.channel,
                ),
                now: levels.now,
              ),
            ),
            for (final t in c.threads)
              LevelTreeRow(
                key: ValueKey('tree.thread.${t.id.value}'),
                depth: 2,
                name: t.title,
                target: LevelMenuTarget(
                  kind: NotificationTargetKind.thread,
                  id: t.id.value,
                  own: levels.own(NotificationTargetKind.thread, t.id.value),
                  inherited: levels.inherited(
                    NotificationTargetKind.thread,
                    channel: c.channel,
                  ),
                  now: levels.now,
                ),
              ),
          ],
        ],
      ],
    );
  }
}

/// One row of the tree: the name at [depth] and the level control of [target].
class LevelTreeRow extends ConsumerWidget {
  /// Creates the row.
  const LevelTreeRow({
    super.key,
    required this.depth,
    required this.name,
    required this.target,
  });

  /// 0 for a space, 1 for a channel, 2 for a thread.
  final int depth;

  /// The shown name.
  final String name;

  /// The target with its own row and inherited level.
  final LevelMenuTarget target;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final writer = ref.watch(notifyWriterProvider);
    final zone = ref.watch(personTimeZoneProvider);
    final now = ref.watch(appPortsProvider).clock.now();
    final own = target.own;
    final until = target.effective.mutedUntil;
    final value = own?.level.code ?? inheritChoice;
    return Padding(
      padding: EdgeInsetsDirectional.only(
        start: DsSpace.s16 * depth,
        top: DsSpace.s2,
        bottom: DsSpace.s2,
      ),
      // One node for screen readers: the name of the target names its level control, and
      // the control says when it cannot be changed (01 8, WCAG 4.1.2; brief M4-32).
      child: MergeSemantics(
        child: Semantics(
          enabled: writer != null,
          // A wrap, so the level control moves below a long name at a large text scale.
          child: Wrap(
            spacing: DsSpace.s8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Text(
                dsIsolate(name),
                style: dsTextStyle(depth == 0 ? DsType.bodyStrong : DsType.body)
                    .copyWith(color: colors.textPrimary),
              ),
              DropdownButton<String>(
                key: const ValueKey('tree.level'),
                value: value,
                onChanged: writer == null
                    ? null
                    : (choice) {
                        if (choice == null || choice == value) return;
                        unawaited(
                          choice == inheritChoice
                              ? writer.inherit(target.kind, target.id)
                              : writer.setLevel(
                                  target.kind,
                                  target.id,
                                  NotificationLevel.fromCode(choice),
                                ),
                        );
                      },
                items: [
                  DropdownMenuItem(
                    value: inheritChoice,
                    child: Text(
                      inheritsLabel(l, target.inherited),
                      key: const ValueKey('tree.choice.inherit'),
                    ),
                  ),
                  for (final level in const [
                    NotificationLevel.all,
                    NotificationLevel.mentions,
                    NotificationLevel.mute,
                  ])
                    DropdownMenuItem(
                      value: level.code,
                      child: Text(
                        target.following && level == NotificationLevel.all
                            ? l.notifyFollowing
                            : levelLabel(l, level),
                        key: ValueKey('tree.choice.${level.code}'),
                      ),
                    ),
                ],
              ),
              if (until != null)
                Text(
                  l.notifyMutedUntil(
                    muteEndLabel(context, until, now: now, timeZone: zone),
                  ),
                  key: const ValueKey('tree.mutedUntil'),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.textSecondary),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
