// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/widgets.dart';
import 'package:vector_graphics/vector_graphics.dart';

import '../generated/design_tokens.g.dart';
import '../generated/ds_icon_assets.g.dart';
import '../theme/ds_theme.dart';

/// The three icon sizes of `icon.size.*`.
enum DsIconSizeRole {
  /// `icon.size.sm` (dense lists, inline with small text).
  sm(DsIconSize.sizeSm),

  /// `icon.size.md`, the default.
  md(DsIconSize.sizeMd),

  /// `icon.size.lg` (headers, empty states).
  lg(DsIconSize.sizeLg);

  const DsIconSizeRole(this.size);

  /// Edge length in logical pixels.
  final double size;
}

/// A semantic icon of [DsIcons], drawn from the compiled Phosphor assets of this package.
///
/// The glyph and its weight come from the token mapping (`icon.tokens.json`); [filled] draws
/// the `fill` form for active and reached states. A glyph with [DsGlyph.spins] rotates once per
/// `motion.duration.pulse`, and only in the full motion context; in reduced motion it stands
/// still and no animation controller exists.
///
/// An icon is never alone (docs/design/components.md): pass a [label] for the screen reader, or
/// `decorative: true` when text next to it already says the same.
///
/// Throws an [ArgumentError] when neither a non-empty [label] nor [decorative] is given, in
/// debug and in release builds.
///
/// ```dart
/// DsIcon(DsIcons.triggerWorking, label: l10n.triggerWorking)
/// DsIcon(DsIcons.actionPin, decorative: true, filled: pinned)
/// ```
class DsIcon extends StatefulWidget {
  /// Creates the icon of [glyph].
  ///
  /// Parameters: [size] is one of the `icon.size.*` tokens; [color] defaults to
  /// `color.text.primary` of the ambient theme; [label] is the semantics label; [decorative]
  /// excludes the icon from semantics; [filled] selects the `fill` asset.
  DsIcon(
    this.glyph, {
    super.key,
    this.size = DsIconSizeRole.md,
    this.color,
    this.label,
    this.decorative = false,
    this.filled = false,
  }) {
    if (!decorative && (label == null || label!.isEmpty)) {
      throw ArgumentError.value(
        label,
        'label',
        'DsIcon(${glyph.name}) needs a label or decorative: true',
      );
    }
  }

  /// The semantic icon.
  final DsGlyph glyph;

  /// The token size.
  final DsIconSizeRole size;

  /// The color, or null for `color.text.primary`.
  final Color? color;

  /// The semantics label, or null for a decorative icon.
  final String? label;

  /// Whether the icon is excluded from semantics.
  final bool decorative;

  /// Whether the `fill` form is drawn instead of the mapped weight.
  final bool filled;

  @override
  State<DsIcon> createState() => DsIconState();
}

/// State of [DsIcon]: owns the spin controller while the glyph spins in full motion.
class DsIconState extends State<DsIcon> with TickerProviderStateMixin {
  AnimationController? _spin;

  /// The asset path the icon draws (relative to this package).
  @visibleForTesting
  String get asset {
    final path = DsIconAssets.lookup(widget.glyph, filled: widget.filled);
    if (path == null) {
      // `make tokens` and icons_complete_test keep every mapped pair in the assets.
      throw StateError(
        'no icon asset for ${widget.glyph.weight.name}/${widget.glyph.name}',
      );
    }
    return path;
  }

  /// The color the icon is drawn in.
  @visibleForTesting
  Color get effectiveColor => widget.color ?? context.dsColors.textPrimary;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _updateSpin();
  }

  @override
  void didUpdateWidget(DsIcon oldWidget) {
    super.didUpdateWidget(oldWidget);
    _updateSpin();
  }

  /// Creates or disposes the controller. The reduced context has a zero pulse, so a spin
  /// needs a positive `motion.duration.pulse`, which only the full context has.
  void _updateSpin() {
    final pulse = context.dsMotion.durationPulse;
    final spins = widget.glyph.spins && pulse > Duration.zero;
    if (!spins) {
      _spin?.dispose();
      _spin = null;
      return;
    }
    final spin = _spin;
    if (spin == null) {
      _spin = AnimationController(vsync: this, duration: pulse)..repeat();
    } else if (spin.duration != pulse) {
      spin
        ..duration = pulse
        ..repeat();
    }
  }

  @override
  void dispose() {
    _spin?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final edge = widget.size.size;
    Widget icon = VectorGraphic(
      loader: AssetBytesLoader(asset, packageName: dsIconPackage),
      width: edge,
      height: edge,
      colorFilter: ColorFilter.mode(effectiveColor, BlendMode.srcIn),
      placeholderBuilder: (_) => DsIconPlaceholder(size: edge),
    );
    final spin = _spin;
    if (spin != null) {
      icon = RotationTransition(turns: spin, child: icon);
    }
    icon = SizedBox.square(dimension: edge, child: icon);
    if (widget.decorative) {
      return ExcludeSemantics(child: icon);
    }
    return Semantics(
      label: widget.label,
      image: true,
      child: ExcludeSemantics(child: icon),
    );
  }
}

/// The empty square a [DsIcon] shows until its asset is loaded.
///
/// Public so that tests can wait until no icon is pending (`dsPumpIcons` of
/// `package:design_system/testing.dart`).
class DsIconPlaceholder extends StatelessWidget {
  /// Creates the placeholder with the edge length [size].
  const DsIconPlaceholder({super.key, required this.size});

  /// Edge length in logical pixels.
  final double size;

  @override
  Widget build(BuildContext context) => SizedBox.square(dimension: size);
}
