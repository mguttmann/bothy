// SPDX-License-Identifier: Apache-2.0

/// The parts every target menu shares (brief M4-33 Design note 2; 13 3.1, 3.2): the region that
/// opens a menu by right click, the menu key or Shift+F10, the submenu "Notifications" with the
/// three levels and "Inherit", the submenu "Mute until" with its fixed choices, and the
/// times of those choices.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:timezone/data/latest_all.dart' as tz_data;
import 'package:timezone/timezone.dart' as tz;

import '../app/providers/ports.dart';
import '../notifications_settings/effective_level.dart';
import '../notifications_settings/providers.dart';
import '../settings/settings_providers.dart';

/// Wraps [child] so that a right click, the menu key or Shift+F10 on it opens a menu of
/// [menuChildren] at the pointer (or at the top left of [child] for the keyboard). On a touch
/// screen a long press opens it where the finger lifts (01 6.18 item 1): the press must end
/// without moving, so a long press that moves stays the drag of a sidebar row (D-M4-51-6), and a
/// tap that follows a long press does not open the row (D-DEL-UI-1).
///
/// The entries are built only while the menu is open, inside a [Consumer] that watches the
/// [levelContextProvider]: the row itself needs no provider (the sidebar and the post list
/// stay plain views), and the open menu follows a change of the rows.
///
/// Example:
/// ```dart
/// TargetMenuRegion(
///   label: l.notifyMenuOf('general'),
///   menuChildren: (context, ref) => [...],
///   child: row,
/// );
/// ```
class TargetMenuRegion extends StatefulWidget {
  /// Creates the region.
  const TargetMenuRegion({
    super.key,
    required this.label,
    required this.menuChildren,
    required this.child,
  });

  /// The screen reader label of the menu.
  final String label;

  /// Builds the entries when the menu opens.
  final List<Widget> Function(BuildContext context, WidgetRef ref) menuChildren;

  /// The row the menu belongs to.
  final Widget child;

  @override
  State<TargetMenuRegion> createState() => _TargetMenuRegionState();
}

class _TargetMenuRegionState extends State<TargetMenuRegion> {
  final MenuController _menu = MenuController();

  /// The finger of a press that may become a long press, where it went down, and whether it
  /// has been held for [kLongPressTimeout].
  int? _finger;
  Offset _fingerAt = Offset.zero;
  bool _held = false;
  Timer? _hold;

  static const Set<PointerDeviceKind> _touchKinds = {
    PointerDeviceKind.touch,
    PointerDeviceKind.stylus,
    PointerDeviceKind.invertedStylus,
  };

  void _open([Offset? at]) => _menu.open(position: at ?? Offset.zero);

  void _down(PointerDownEvent e) {
    if (e.buttons == kSecondaryMouseButton) {
      _open(e.localPosition);
      return;
    }
    if (!_touchKinds.contains(e.kind)) return;
    _release();
    _finger = e.pointer;
    _fingerAt = e.localPosition;
    _hold = Timer(kLongPressTimeout, () => _held = true);
  }

  void _move(PointerMoveEvent e) {
    if (e.pointer != _finger) return;
    if ((e.localPosition - _fingerAt).distance > kTouchSlop) _release();
  }

  void _up(PointerUpEvent e) {
    if (e.pointer != _finger) return;
    final open = _held;
    _release();
    if (open) _open(_fingerAt);
  }

  void _release() {
    _hold?.cancel();
    _hold = null;
    _finger = null;
    _held = false;
  }

  @override
  void dispose() {
    _hold?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => MenuAnchor(
    controller: _menu,
    menuChildren: [
      Semantics(
        container: true,
        label: widget.label,
        explicitChildNodes: true,
        child: Consumer(
          builder: (_, ref, _) {
            ref.watch(levelContextProvider);
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              // The region's own context: the menu's context is gone once the menu closes,
              // and an entry opens a dialog (date picker, export) after that.
              children: widget.menuChildren(context, ref),
            );
          },
        ),
      ),
    ],
    child: CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.f10, shift: true): _open,
        const SingleActivator(LogicalKeyboardKey.contextMenu): _open,
      },
      child: Listener(
        // Raw pointer events, outside the gesture arena: the row's own recognizer must not
        // swallow the right click, and a drag that wins the long press still opens the menu
        // when the finger lifts where it went down.
        onPointerDown: _down,
        onPointerMove: _move,
        onPointerUp: _up,
        onPointerCancel: (e) {
          if (e.pointer == _finger) _release();
        },
        child: RawGestureDetector(
          // Takes a held touch from the row's tap, so lifting the finger opens only the menu.
          // Later than [kLongPressTimeout], so a drag or the note of a sidebar row wins first.
          // No node of its own: the row keeps its one labelled node (01 8).
          excludeFromSemantics: true,
          gestures: {
            LongPressGestureRecognizer:
                GestureRecognizerFactoryWithHandlers<
                  LongPressGestureRecognizer
                >(
                  () => LongPressGestureRecognizer(
                    duration: kLongPressTimeout + _holdMargin,
                    supportedDevices: _touchKinds,
                  ),
                  (r) => r.onLongPress = _noop,
                ),
          },
          child: widget.child,
        ),
      ),
    ),
  );
}

/// How much later than [kLongPressTimeout] the menu region claims a held touch.
const Duration _holdMargin = Duration(milliseconds: 50);

void _noop() {}

/// The times of the fixed "Mute until" choices (Design note 2; docs/DECISIONS.md D-M4-33-2):
/// one hour from [now], and tomorrow at 08:00 in the person's [timeZone] (IANA name; an
/// unknown zone counts as UTC, as the server treats a zone that no longer loads, D-M3-18-8).
///
/// Example: `muteChoices(DateTime.utc(2026, 10, 20, 21), 'Europe/Berlin').tomorrow` is
/// 2026-10-21 06:00 UTC.
({DateTime oneHour, DateTime tomorrow}) muteChoices(
  DateTime now,
  String timeZone,
) {
  final zone = _location(timeZone);
  final local = tz.TZDateTime.from(now, zone);
  final tomorrow = tz.TZDateTime(
    zone,
    local.year,
    local.month,
    local.day + 1,
    8,
  );
  return (
    oneHour: now.toUtc().add(const Duration(hours: 1)),
    tomorrow: tomorrow.toUtc(),
  );
}

/// The instant of [day] at [time] in [timeZone], for the choice "Choose date and time".
DateTime instantIn(String timeZone, DateTime day, TimeOfDay time) =>
    tz.TZDateTime(
      _location(timeZone),
      day.year,
      day.month,
      day.day,
      time.hour,
      time.minute,
    ).toUtc();

bool _tzLoaded = false;

tz.Location _location(String name) {
  if (!_tzLoaded) {
    tz_data.initializeTimeZones();
    _tzLoaded = true;
  }
  try {
    return tz.getLocation(name);
  } on tz.LocationNotFoundException {
    return tz.UTC;
  }
}

/// What the level part of a menu shows for one target.
final class LevelMenuTarget {
  /// Creates the description of target [kind] [id].
  const LevelMenuTarget({
    required this.kind,
    required this.id,
    required this.own,
    required this.inherited,
    required this.now,
  });

  /// The kind of target.
  final NotificationTargetKind kind;

  /// Its ID.
  final String id;

  /// The person's own row, or null while it inherits.
  final NotificationPreference? own;

  /// What it inherits without [own].
  final EffectiveLevel inherited;

  /// Now, for an active `muted_until`.
  final DateTime now;

  /// The level that applies now with [own] (13 3.1).
  EffectiveLevel get effective {
    final row = own;
    if (row == null) return inherited;
    final until = row.mutedUntil;
    final muted = until != null && until.isAfter(now);
    return EffectiveLevel(
      level: muted ? NotificationLevel.mute : row.level,
      source: switch (kind) {
        NotificationTargetKind.space => LevelSource.space,
        NotificationTargetKind.channel => LevelSource.channel,
        _ => LevelSource.thread,
      },
      storedLevel: row.level,
      mutedUntil: muted ? until : null,
    );
  }

  /// Whether the server follows the thread for the person: an own thread row at `all`
  /// without an active mute, which is what 13 3.2 writes (shown as "Following").
  bool get following =>
      kind == NotificationTargetKind.thread &&
      own?.level == NotificationLevel.all &&
      effective.mutedUntil == null;

  /// The level "Mute until" keeps for the time after it: the own row's level, else the one
  /// it inherits now (D-M4-33-2).
  NotificationLevel get levelAfterMute => own?.level ?? inherited.level;
}

/// The state text of [target]: "Following", "muted until ...", the own level or
/// "inherits: ...".
String levelStateLabel(
  AppLocalizations l,
  LevelMenuTarget target, {
  required String Function(DateTime) when,
}) {
  final effective = target.effective;
  if (effective.mutedUntil case final until?) {
    return l.notifyMutedUntil(when(until));
  }
  if (target.following) return l.notifyFollowing;
  if (target.own case final own?) return levelLabel(l, own.level);
  return inheritsLabel(l, target.inherited);
}

/// The time label of a mute end [at]: the relative day and the clock time in [timeZone].
String muteEndLabel(
  BuildContext context,
  DateTime at, {
  required DateTime now,
  required String timeZone,
}) {
  final format = L10nFormat(Localizations.localeOf(context));
  return '${format.relativeDay(at, now: now, timeZone: timeZone)} '
      '${format.time(at, timeZone: timeZone)}';
}

/// The submenus "Notifications" and "Mute until" of [target]; the writes go through
/// [notifyWriterProvider] and change the local row at once.
List<Widget> levelMenuItems(
  BuildContext context,
  WidgetRef ref,
  LevelMenuTarget target,
) {
  final l = AppLocalizations.of(context);
  final sections = levelMenuSections(context, ref, target);
  return [
    SubmenuButton(
      key: const ValueKey('menu.notifications'),
      menuChildren: sections.levels,
      child: Text(l.notifyMenuNotifications),
    ),
    SubmenuButton(
      key: const ValueKey('menu.muteUntil'),
      menuChildren: sections.mute,
      child: Text(l.notifyMenuMuteUntil),
    ),
  ];
}

/// The entries of the two submenus of [target]: [levels] holds the state ("Following",
/// "inherits: ...", "muted until ..."), the three levels and "Inherit"; [mute] the fixed
/// choices of "Mute until".
({List<Widget> levels, List<Widget> mute}) levelMenuSections(
  BuildContext context,
  WidgetRef ref,
  LevelMenuTarget target,
) {
  final l = AppLocalizations.of(context);
  final writer = ref.read(notifyWriterProvider);
  final zone = ref.read(personTimeZoneProvider);
  final now = ref.read(appPortsProvider).clock.now();
  String when(DateTime at) =>
      muteEndLabel(context, at, now: now, timeZone: zone);
  final choices = muteChoices(now, zone);
  final own = target.own;
  void set(NotificationLevel level, {DateTime? until}) {
    if (writer == null) return;
    unawaited(
      writer.setLevel(target.kind, target.id, level, mutedUntil: until),
    );
  }

  Future<void> chooseDate() async {
    final day = await showDatePicker(
      context: context,
      firstDate: now,
      lastDate: now.add(const Duration(days: 365)),
    );
    if (day == null || !context.mounted) return;
    final time = await showTimePicker(
      context: context,
      initialTime: const TimeOfDay(hour: 8, minute: 0),
    );
    if (time == null) return;
    set(target.levelAfterMute, until: instantIn(zone, day, time));
  }

  return (
    levels: [
      // The state is information, not an inactive entry: text in `color.text.secondary`,
      // which meets the contrast of 01 8 (a disabled entry's color does not; brief M4-32).
      Padding(
        key: const ValueKey('menu.state'),
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s12,
          vertical: DsSpace.s8,
        ),
        child: Text(
          levelStateLabel(l, target, when: when),
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: context.dsColors.textSecondary),
        ),
      ),
      for (final level in const [
        NotificationLevel.all,
        NotificationLevel.mentions,
        NotificationLevel.mute,
      ])
        DsMenuItem(
          key: ValueKey('menu.level.${level.code}'),
          // The check mark is not read; the chosen level says "selected" (brief M4-32).
          selected: own?.level == level && own?.mutedUntil == null,
          leadingIcon: Icon(
            own?.level == level && own?.mutedUntil == null ? Icons.check : null,
            size: 16,
          ),
          onPressed: writer == null ? null : () => set(level),
          child: Text(levelLabel(l, level)),
        ),
      DsMenuItem(
        key: const ValueKey('menu.inherit'),
        selected: own == null,
        onPressed: writer == null || own == null
            ? null
            : () => unawaited(writer.inherit(target.kind, target.id)),
        child: Text(
          l.notifyInheritFrom(
            levelLabel(l, target.inherited.level),
            levelSourceLabel(l, target.inherited.source),
          ),
        ),
      ),
    ],
    mute: [
      DsMenuItem(
        key: const ValueKey('menu.mute.hour'),
        onPressed: writer == null
            ? null
            : () => set(target.levelAfterMute, until: choices.oneHour),
        child: Text(l.notifyMuteOneHour),
      ),
      DsMenuItem(
        key: const ValueKey('menu.mute.tomorrow'),
        onPressed: writer == null
            ? null
            : () => set(target.levelAfterMute, until: choices.tomorrow),
        child: Text(
          l.notifyMuteTomorrow(
            L10nFormat(Localizations.localeOf(context))
                .time(choices.tomorrow, timeZone: zone),
          ),
        ),
      ),
      DsMenuItem(
        key: const ValueKey('menu.mute.date'),
        onPressed: writer == null ? null : () => unawaited(chooseDate()),
        child: Text(l.notifyMuteChooseDate),
      ),
    ],
  );
}
