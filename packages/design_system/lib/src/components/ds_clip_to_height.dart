// SPDX-License-Identifier: Apache-2.0

/// Keeps at most a given height of a child laid out at its natural height: the collapse of a
/// long message ("Show more", 01 6.4) and of a long code block ("Show all", 01 6.14).
library;

import 'package:flutter/rendering.dart';
import 'package:flutter/widgets.dart';

/// Lays [child] out at its natural height and keeps at most [maxHeight] of it, clipped, while
/// [clip] is true; reports through [onOverflow] after each layout whether the natural height is
/// above [maxHeight], also while [clip] is false, so a caller can offer both "expand" and
/// "collapse".
///
/// The measure is the rendered height, so a long line that wraps counts as several lines. A
/// caller changes its state from [onOverflow] only in a post-frame callback, because the
/// report comes during layout.
///
/// ```dart
/// DsClipToHeight(maxHeight: 200, onOverflow: (o) => schedule(o), child: text);
/// ```
class DsClipToHeight extends SingleChildRenderObjectWidget {
  /// Creates the clip.
  const DsClipToHeight({
    super.key,
    required this.maxHeight,
    required this.onOverflow,
    this.clip = true,
    required super.child,
  });

  /// The highest height kept while [clip] is true.
  final double maxHeight;

  /// Called after each layout with whether the natural height is above [maxHeight].
  final ValueChanged<bool> onOverflow;

  /// Whether the child is cut to [maxHeight]; false shows it whole.
  final bool clip;

  @override
  RenderObject createRenderObject(BuildContext context) =>
      _RenderClipToHeight(maxHeight, onOverflow, clip);

  @override
  void updateRenderObject(BuildContext context, RenderObject renderObject) {
    (renderObject as _RenderClipToHeight)
      ..maxHeight = maxHeight
      ..onOverflow = onOverflow
      ..clip = clip;
  }
}

class _RenderClipToHeight extends RenderProxyBox {
  _RenderClipToHeight(this._maxHeight, this.onOverflow, this._clip);

  double _maxHeight;
  ValueChanged<bool> onOverflow;
  bool _clip;

  set maxHeight(double value) {
    if (value == _maxHeight) return;
    _maxHeight = value;
    markNeedsLayout();
  }

  set clip(bool value) {
    if (value == _clip) return;
    _clip = value;
    markNeedsLayout();
  }

  bool _clipped = false;

  @override
  void performLayout() {
    final box = child!;
    box.layout(
      constraints.copyWith(maxHeight: double.infinity),
      parentUsesSize: true,
    );
    final overflows = box.size.height > _maxHeight;
    _clipped = _clip && overflows;
    size = constraints.constrain(
      Size(box.size.width, _clipped ? _maxHeight : box.size.height),
    );
    onOverflow(overflows);
  }

  @override
  void paint(PaintingContext context, Offset offset) {
    if (!_clipped) {
      layer = null;
      super.paint(context, offset);
      return;
    }
    layer = context.pushClipRect(
      needsCompositing,
      offset,
      Offset.zero & size,
      super.paint,
      oldLayer: layer as ClipRectLayer?,
    );
  }

  @override
  bool hitTest(BoxHitTestResult result, {required Offset position}) {
    if (!size.contains(position)) return false;
    return super.hitTest(result, position: position);
  }
}
