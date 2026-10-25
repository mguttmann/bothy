// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';

import '../generated/design_tokens.g.dart';
import '../theme/ds_theme.dart';

/// The visible focus of the whole app: a ring in `color.focus.ring` and `border-width.focus`,
/// offset outside the control that has the keyboard focus (01 8 "Focus", WCAG 2.4.7;
/// components.md "Common rules"; brief M4-32).
///
/// Material controls show their focus as a faint overlay (icon buttons, switches, check
/// boxes, list tiles, drop-down buttons, segmented buttons) or as a border in the color of
/// their own fill (filled buttons), which is not a visible focus. This widget draws one ring
/// for every focused control at one place, above the app and its menus and dialogs, while the
/// person works with the keyboard: the ring shows after a key press and hides after a press of
/// a pointer, so a click, a hover that moves the focus of a menu or the start focus of a screen
/// draws no ring (as `:focus-visible` on the web). Touch input
/// ([FocusHighlightMode.touch]) never shows it.
///
/// Controls that draw the ring themselves keep theirs alone: a `DsTappable` (marked with
/// [DsDrawsFocusRing]), text and outlined buttons (the theme's focus side), and text fields
/// (the focused border).
///
/// Install it once below the [DsThemeScope], around the app's navigator, for example in
/// `MaterialApp.builder`:
///
/// ```dart
/// builder: (context, child) => DsThemeScope(
///   dsContext: ds,
///   child: DsFocusRing(child: child!),
/// ),
/// ```
class DsFocusRing extends StatefulWidget {
  /// Draws the ring above [child].
  const DsFocusRing({super.key, required this.child});

  /// The app.
  final Widget child;

  @override
  State<DsFocusRing> createState() => _DsFocusRingState();
}

class _DsFocusRingState extends State<DsFocusRing> {
  final GlobalKey _area = GlobalKey();
  Rect? _ring;
  bool _scheduled = false;

  /// Whether the last input was a key: the ring shows only then.
  bool _keyboard = false;

  @override
  void initState() {
    super.initState();
    FocusManager.instance.addListener(_changed);
    FocusManager.instance.addHighlightModeListener(_modeChanged);
    HardwareKeyboard.instance.addHandler(_key);
    GestureBinding.instance.pointerRouter.addGlobalRoute(_pointer);
    _changed();
  }

  @override
  void dispose() {
    FocusManager.instance.removeListener(_changed);
    FocusManager.instance.removeHighlightModeListener(_modeChanged);
    HardwareKeyboard.instance.removeHandler(_key);
    GestureBinding.instance.pointerRouter.removeGlobalRoute(_pointer);
    super.dispose();
  }

  bool _key(KeyEvent event) {
    if (event is KeyDownEvent && !_keyboard) {
      _keyboard = true;
      _changed();
    }
    return false;
  }

  void _pointer(PointerEvent event) {
    if (event is PointerDownEvent && _keyboard) {
      _keyboard = false;
      _changed();
    }
  }

  void _modeChanged(FocusHighlightMode _) => _changed();

  /// Measures the focused control after the next frame, which a change of the focus asks for.
  void _changed() {
    _watch();
    SchedulerBinding.instance.ensureVisualUpdate();
  }

  /// While a ring shows, it is measured again after every frame that happens anyway, so it
  /// follows scrolling and layout; it asks for no frame of its own, so an idle app stays idle.
  void _watch() {
    if (_scheduled) return;
    _scheduled = true;
    SchedulerBinding.instance.addPostFrameCallback((_) {
      _scheduled = false;
      if (!mounted) return;
      final ring = _measure();
      if (ring != _ring) setState(() => _ring = ring);
      if (ring != null) _watch();
    });
  }

  Rect? _measure() {
    if (!_keyboard ||
        FocusManager.instance.highlightMode != FocusHighlightMode.traditional) {
      return null;
    }
    final node = FocusManager.instance.primaryFocus;
    final context = node?.context;
    if (node == null || node is FocusScopeNode || context == null) return null;
    if (!context.mounted || _drawsOwnRing(context)) return null;
    final target = context.findRenderObject();
    final area = _area.currentContext?.findRenderObject();
    if (target is! RenderBox ||
        area is! RenderBox ||
        !target.attached ||
        !target.hasSize ||
        target.size.isEmpty) {
      return null;
    }
    return MatrixUtils.transformRect(
      target.getTransformTo(area),
      Offset.zero & target.size,
    );
  }

  static bool _drawsOwnRing(BuildContext context) {
    if (context.widget is EditableText) return true;
    var marked = false;
    var themedButton = false;
    var inComposite = false;
    // Only the control around the focus node counts: the walk ends at the next Focus.
    context.visitAncestorElements((element) {
      final widget = element.widget;
      if (widget is DsDrawsFocusRing || widget is EditableText) marked = true;
      // Text and outlined buttons draw the theme's focus side; inside a segmented button or
      // a menu entry they are styled by those, which draw none.
      if (widget is TextButton || widget is OutlinedButton) themedButton = true;
      if (widget is SegmentedButton ||
          widget is MenuItemButton ||
          widget is SubmenuButton) {
        inComposite = true;
      }
      return widget is! Focus && !marked;
    });
    return marked || (themedButton && !inComposite);
  }

  @override
  Widget build(BuildContext context) {
    final ring = _ring;
    return Stack(
      key: _area,
      fit: StackFit.passthrough,
      children: [
        widget.child,
        if (ring != null)
          Positioned.fill(
            child: IgnorePointer(
              child: CustomPaint(
                key: const ValueKey('ds.focusRing'),
                painter: _RingPainter(ring, context.dsAccent.focusRing),
              ),
            ),
          ),
      ],
    );
  }
}

/// Marks a control that draws the focus ring itself, so [DsFocusRing] draws none around it.
class DsDrawsFocusRing extends StatelessWidget {
  /// Marks [child].
  const DsDrawsFocusRing({super.key, required this.child});

  /// The control.
  final Widget child;

  @override
  Widget build(BuildContext context) => child;
}

class _RingPainter extends CustomPainter {
  _RingPainter(this.rect, this.color);

  final Rect rect;
  final Color color;

  /// The gap between the control and the ring, so the ring stands apart from a fill in the
  /// same color (a filled button in the accent color).
  static const double _gap = DsBorderWidth.focus;

  @override
  void paint(Canvas canvas, Size size) {
    final outer = rect.inflate(_gap + DsBorderWidth.focus / 2);
    canvas.drawRRect(
      RRect.fromRectAndRadius(outer, const Radius.circular(DsRadius.md)),
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = DsBorderWidth.focus
        ..color = color,
    );
  }

  @override
  bool shouldRepaint(_RingPainter old) =>
      old.rect != rect || old.color != color;
}
