// SPDX-License-Identifier: Apache-2.0

/// The status mark of unseen agent news on the rows of the sidebar (01 6.17, components.md 7):
/// a chat whose agent state changed since the person last had it open shows a small mark in the
/// colour and shape of the new state; a forum or project row shows the most urgent one of its
/// chats.
library;

import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../generated/design_tokens.g.dart';
import '../theme/ds_theme.dart';

/// The unseen agent state of a chat (01 6.17 item 1), ordered by [urgency].
enum DsChatStatus {
  /// A run finished (green).
  done(0),

  /// A run is working (yellow).
  working(1),

  /// The agent waits for the person: a question, an approval card, an input request (blue).
  waiting(2),

  /// A run failed (red).
  failed(3);

  const DsChatStatus(this.urgency);

  /// The rank of the state: a higher one wins in an aggregate (01 6.17 item 6: red over blue
  /// over yellow over green).
  final int urgency;

  /// The most urgent of [statuses], ignoring nulls; null when none is given.
  ///
  /// ```dart
  /// DsChatStatus.mostUrgent([DsChatStatus.done, null, DsChatStatus.failed]); // failed
  /// ```
  static DsChatStatus? mostUrgent(Iterable<DsChatStatus?> statuses) {
    DsChatStatus? best;
    for (final s in statuses) {
      if (s != null && (best == null || s.urgency > best.urgency)) best = s;
    }
    return best;
  }
}

/// The shape of a [DsChatStatusMark]; each state has its own, so a state never rests on colour
/// alone (01 8 "Color").
enum DsChatStatusShape {
  /// A filled circle ([DsChatStatus.done]).
  dot,

  /// A circle outline ([DsChatStatus.working]): the run is not finished.
  ring,

  /// A filled diamond ([DsChatStatus.waiting]).
  diamond,

  /// A filled triangle, point up ([DsChatStatus.failed]), the warning shape.
  triangle,
}

/// The words of [status] for the tooltip and the screen reader label (01 6.17 item 5).
String dsChatStatusLabel(AppLocalizations l, DsChatStatus status) =>
    switch (status) {
      DsChatStatus.done => l.chatStatusDone,
      DsChatStatus.working => l.chatStatusWorking,
      DsChatStatus.waiting => l.chatStatusWaiting,
      DsChatStatus.failed => l.chatStatusFailed,
    };

/// The mark of [status] at `size.status-dot` (components.md 7).
///
/// Colours are the `on-sidebar` steps of the categories, which are paired with every sidebar
/// surface (design-rules.json): done `category.moss.on-sidebar`, working
/// `category.gorse.on-sidebar`, waiting `category.loch.on-sidebar`, failed
/// `category.rowan.on-sidebar`. The shapes ([DsChatStatusShape]) differ for every state. The
/// tooltip names the state ([dsChatStatusLabel]); the mark has no screen reader node of its own,
/// because the row ends its label with the same words.
///
/// ```dart
/// DsChatStatusMark(DsChatStatus.waiting)
/// ```
class DsChatStatusMark extends StatefulWidget {
  /// Creates the mark of [status].
  const DsChatStatusMark(this.status, {super.key});

  /// The state the mark shows.
  final DsChatStatus status;

  @override
  State<DsChatStatusMark> createState() => DsChatStatusMarkState();
}

/// State of [DsChatStatusMark]; exposes the resolved colour and shape to tests.
class DsChatStatusMarkState extends State<DsChatStatusMark> {
  /// The colour of the mark.
  @visibleForTesting
  Color get color {
    final c = context.dsColors;
    return switch (widget.status) {
      DsChatStatus.done => c.categoryMossOnSidebar,
      DsChatStatus.working => c.categoryGorseOnSidebar,
      DsChatStatus.waiting => c.categoryLochOnSidebar,
      DsChatStatus.failed => c.categoryRowanOnSidebar,
    };
  }

  /// The shape of the mark.
  @visibleForTesting
  DsChatStatusShape get shape => switch (widget.status) {
    DsChatStatus.done => DsChatStatusShape.dot,
    DsChatStatus.working => DsChatStatusShape.ring,
    DsChatStatus.waiting => DsChatStatusShape.diamond,
    DsChatStatus.failed => DsChatStatusShape.triangle,
  };

  @override
  Widget build(BuildContext context) => Tooltip(
    message: dsChatStatusLabel(AppLocalizations.of(context), widget.status),
    excludeFromSemantics: true,
    child: SizedBox.square(
      dimension: DsSize.statusDot,
      child: CustomPaint(painter: _StatusPainter(shape, color)),
    ),
  );
}

class _StatusPainter extends CustomPainter {
  const _StatusPainter(this.shape, this.color);

  final DsChatStatusShape shape;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final side = math.min(size.width, size.height);
    final center = size.center(Offset.zero);
    final fill = Paint()
      ..color = color
      ..isAntiAlias = true;
    switch (shape) {
      case DsChatStatusShape.dot:
        canvas.drawCircle(center, side / 2, fill);
      case DsChatStatusShape.ring:
        // A stroke of a quarter of the side keeps the ring readable at `size.status-dot`.
        final stroke = side / 4;
        canvas.drawCircle(
          center,
          (side - stroke) / 2,
          fill
            ..style = PaintingStyle.stroke
            ..strokeWidth = stroke,
        );
      case DsChatStatusShape.diamond:
        final r = side / 2;
        canvas.drawPath(
          Path()
            ..moveTo(center.dx, center.dy - r)
            ..lineTo(center.dx + r, center.dy)
            ..lineTo(center.dx, center.dy + r)
            ..lineTo(center.dx - r, center.dy)
            ..close(),
          fill,
        );
      case DsChatStatusShape.triangle:
        final top = center.dy - side / 2;
        canvas.drawPath(
          Path()
            ..moveTo(center.dx, top)
            ..lineTo(center.dx + side / 2, top + side)
            ..lineTo(center.dx - side / 2, top + side)
            ..close(),
          fill,
        );
    }
  }

  @override
  bool shouldRepaint(_StatusPainter old) =>
      old.shape != shape || old.color != color;
}
