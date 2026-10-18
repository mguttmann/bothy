// SPDX-License-Identifier: Apache-2.0

/// Codes and remaining times on the onboarding screens (docs/spec/01-produkt.md 5.6 and 8;
/// brief M4-18 Design note 6; docs/DECISIONS.md D-M4-18-6).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:l10n/l10n.dart';

export 'package:admin_ui/admin_ui.dart' show CodeText, groupCode, spellCode;

/// The remaining time until [deadline] as text ("Valid for 23 more minutes", 01 5.6),
/// announced to screen readers at most once per [announceInterval]
/// (`a11y.live_announce_min_interval`) through the [LiveAnnouncer] of the design system.
///
/// [clock] is injected so tests control the time; [announce] replaces the system announcement
/// in tests.
class RemainingTimeText extends StatefulWidget {
  /// Creates the text.
  const RemainingTimeText({
    super.key,
    required this.deadline,
    required this.clock,
    required this.announceInterval,
    this.announce,
  });

  /// When the code stops being valid.
  final DateTime deadline;

  /// The current time.
  final DateTime Function() clock;

  /// `a11y.live_announce_min_interval`.
  final Duration announceInterval;

  /// Receives each announcement; null sends it to the system.
  final void Function(String message)? announce;

  @override
  State<RemainingTimeText> createState() => _RemainingTimeTextState();
}

class _RemainingTimeTextState extends State<RemainingTimeText> {
  static const Duration _unit = Duration(minutes: 1);
  late LiveAnnouncer _announcer;
  Timer? _tick;
  Timer? _flush;

  @override
  void initState() {
    super.initState();
    _announcer = _newAnnouncer();
  }

  LiveAnnouncer _newAnnouncer() => LiveAnnouncer(
    minInterval: widget.announceInterval,
    clock: widget.clock,
    sink: _send,
  );

  void _send(String message) {
    final announce = widget.announce;
    if (announce != null) {
      announce(message);
      return;
    }
    unawaited(
      SemanticsService.sendAnnouncement(
        View.of(context),
        message,
        Directionality.of(context),
      ),
    );
  }

  @override
  void didUpdateWidget(RemainingTimeText old) {
    super.didUpdateWidget(old);
    // A new deadline (a new pairing code) is a change like any other and goes through the same
    // throttle; only a new interval needs a new announcer.
    if (old.announceInterval != widget.announceInterval) {
      _announcer = _newAnnouncer();
    }
  }

  @override
  void dispose() {
    _tick?.cancel();
    _flush?.cancel();
    super.dispose();
  }

  int _minutes() {
    final left = widget.deadline.difference(widget.clock());
    return left.isNegative ? 0 : left.inMinutes;
  }

  void _schedule() {
    _tick?.cancel();
    final left = widget.deadline.difference(widget.clock());
    if (left.isNegative) return;
    // The text changes at the next whole minute of the remaining time.
    final wait = Duration(
      microseconds: left.inMicroseconds % _unit.inMicroseconds,
    );
    _tick = Timer(wait == Duration.zero ? _unit : wait, () {
      if (mounted) setState(() {});
    });
  }

  @override
  Widget build(BuildContext context) {
    final text = AppLocalizations.of(context).onboardingValidFor(_minutes());
    _announcer.observe(state: text, questionId: null);
    final due = _announcer.pendingDue;
    _flush?.cancel();
    if (due != null) {
      _flush = Timer(due.difference(widget.clock()), _announcer.flush);
    }
    _schedule();
    return Text(
      text,
      key: const ValueKey('remaining.time'),
      textAlign: TextAlign.center,
      style: dsTextStyle(DsType.body)
          .copyWith(color: context.dsColors.textSecondary),
    );
  }
}
