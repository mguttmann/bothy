// SPDX-License-Identifier: Apache-2.0

/// The banner for info, warning and danger notes (live box stall, offline, card errors).
library;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';

/// A note on a tinted surface with the icon of its [tone] and a text, optionally with one
/// action.
///
/// Info, warning and danger differ in icon shape and text, not only in color. The banner is a
/// semantics container with [text] as its label; [liveRegion] makes screen readers announce it
/// when it appears.
///
/// ```dart
/// DsBanner(tone: DsTone.warning, text: l.liveBoxStall(4))
/// ```
class DsBanner extends StatelessWidget {
  /// Creates the banner.
  const DsBanner({
    super.key,
    required this.tone,
    required this.text,
    this.actionLabel,
    this.onAction,
    this.liveRegion = false,
  });

  /// Info, warning or danger.
  final DsTone tone;

  /// The note.
  final String text;

  /// The label of the action, or null for none.
  final String? actionLabel;

  /// The action, or null.
  final VoidCallback? onAction;

  /// Whether the banner is a polite live region.
  final bool liveRegion;

  @override
  Widget build(BuildContext context) {
    final t = dsToneColors(context, tone);
    final style = dsTextStyle(DsType.bodySm).copyWith(color: t.fg);
    return Semantics(
      container: true,
      liveRegion: liveRegion,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s12,
          vertical: DsSpace.s8,
        ),
        decoration: BoxDecoration(
          color: t.subtle,
          border: Border.all(color: t.border, width: DsBorderWidth.hairline),
          borderRadius: BorderRadius.circular(DsRadius.md),
        ),
        child: _BannerLayout(
          children: [
            Row(
              children: [
                DsIcon(
                  dsToneGlyph(tone),
                  size: DsIconSizeRole.sm,
                  color: t.fg,
                  decorative: true,
                ),
                const SizedBox(width: DsSpace.s8),
                Expanded(child: Text(text, style: style)),
              ],
            ),
            if (actionLabel != null && onAction != null)
              DsTappable(
                onTap: onAction,
                label: actionLabel,
                excludeChildSemantics: true,
                child: Center(
                  widthFactor: 1,
                  heightFactor: 1,
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
                    child: Text(
                      actionLabel!,
                      style: dsTextStyle(DsType.label).copyWith(color: t.fg),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// The note and, if given, the action of a [DsBanner]: side by side while the action takes at
/// most a third of the width, the action below the note, at the end, when it is wider (large
/// text, a long word), so the note never shrinks to a column of a few characters (brief
/// M5-10; WCAG 1.4.4 and 1.4.10).
///
/// A render object of its own instead of a `LayoutBuilder`, so the banner still reports its
/// intrinsic sizes to a table or an `IntrinsicHeight` around it.
class _BannerLayout extends MultiChildRenderObjectWidget {
  const _BannerLayout({required super.children});

  @override
  RenderObject createRenderObject(BuildContext context) =>
      _RenderBannerLayout(Directionality.of(context));

  @override
  void updateRenderObject(
    BuildContext context,
    _RenderBannerLayout renderObject,
  ) => renderObject.textDirection = Directionality.of(context);
}

class _BannerParentData extends ContainerBoxParentData<RenderBox> {}

class _RenderBannerLayout extends RenderBox
    with
        ContainerRenderObjectMixin<RenderBox, _BannerParentData>,
        RenderBoxContainerDefaultsMixin<RenderBox, _BannerParentData> {
  _RenderBannerLayout(this._textDirection);

  TextDirection _textDirection;

  set textDirection(TextDirection value) {
    if (value == _textDirection) return;
    _textDirection = value;
    markNeedsLayout();
  }

  @override
  void setupParentData(RenderBox child) {
    if (child.parentData is! _BannerParentData) {
      child.parentData = _BannerParentData();
    }
  }

  RenderBox get _note => firstChild!;

  RenderBox? get _action => childCount > 1 ? lastChild : null;

  /// Whether the action goes below the note at [width].
  bool _stacked(double width) {
    final action = _action;
    return action != null &&
        width.isFinite &&
        action.getMaxIntrinsicWidth(double.infinity) > width / 3;
  }

  @override
  double computeMinIntrinsicWidth(double height) => [
    _note.getMinIntrinsicWidth(height),
    _action?.getMinIntrinsicWidth(height) ?? 0,
  ].reduce((a, b) => a > b ? a : b);

  @override
  double computeMaxIntrinsicWidth(double height) =>
      _note.getMaxIntrinsicWidth(height) +
      (_action?.getMaxIntrinsicWidth(height) ?? 0);

  double _height(
    double width,
    double Function(RenderBox child, double width) height,
  ) {
    final action = _action;
    if (action == null) return height(_note, width);
    if (_stacked(width)) return height(_note, width) + height(action, width);
    final actionWidth = action.getMaxIntrinsicWidth(double.infinity);
    final noteHeight = height(_note, width - actionWidth);
    final actionHeight = height(action, actionWidth);
    return noteHeight > actionHeight ? noteHeight : actionHeight;
  }

  @override
  double computeMinIntrinsicHeight(double width) =>
      _height(width, (child, w) => child.getMinIntrinsicHeight(w));

  @override
  double computeMaxIntrinsicHeight(double width) =>
      _height(width, (child, w) => child.getMaxIntrinsicHeight(w));

  /// The sizes and offsets of note and action in [constraints]; [layout] lays a child out (or
  /// measures it dry) and returns its size.
  (Size, Offset, Offset) _arrange(
    BoxConstraints constraints,
    Size Function(RenderBox child, BoxConstraints constraints) layout,
  ) {
    final width = constraints.maxWidth;
    final action = _action;
    if (action == null) {
      final note = layout(_note, constraints.loosen());
      return (constraints.constrain(note), Offset.zero, Offset.zero);
    }
    if (_stacked(width)) {
      final note = layout(_note, BoxConstraints.tightFor(width: width));
      final act = layout(action, BoxConstraints(maxWidth: width));
      final x = _textDirection == TextDirection.rtl ? 0.0 : width - act.width;
      return (
        constraints.constrain(Size(width, note.height + act.height)),
        Offset.zero,
        Offset(x, note.height),
      );
    }
    final act = layout(
      action,
      BoxConstraints(maxWidth: width.isFinite ? width : double.infinity),
    );
    final noteWidth = width.isFinite ? width - act.width : null;
    final note = layout(
      _note,
      noteWidth == null
          ? const BoxConstraints()
          : BoxConstraints.tightFor(width: noteWidth),
    );
    final height = note.height > act.height ? note.height : act.height;
    final total = constraints.constrain(Size(note.width + act.width, height));
    final rtl = _textDirection == TextDirection.rtl;
    return (
      total,
      Offset(rtl ? act.width : 0, (height - note.height) / 2),
      Offset(rtl ? 0 : note.width, (height - act.height) / 2),
    );
  }

  @override
  Size computeDryLayout(covariant BoxConstraints constraints) =>
      _arrange(constraints, (child, c) => child.getDryLayout(c)).$1;

  @override
  void performLayout() {
    final (total, note, action) = _arrange(constraints, (child, c) {
      child.layout(c, parentUsesSize: true);
      return child.size;
    });
    size = total;
    (_note.parentData! as _BannerParentData).offset = note;
    final a = _action;
    if (a != null) (a.parentData! as _BannerParentData).offset = action;
  }

  @override
  void paint(PaintingContext context, Offset offset) =>
      defaultPaint(context, offset);

  @override
  bool hitTestChildren(BoxHitTestResult result, {required Offset position}) =>
      defaultHitTestChildren(result, position: position);
}
