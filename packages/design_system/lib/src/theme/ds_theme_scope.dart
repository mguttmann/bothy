// SPDX-License-Identifier: Apache-2.0

/// The widget that applies a [DsContext] below it.
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import 'ds_context.dart';
import 'ds_theme.dart';

/// Applies the theme of [dsContext] to [child] with an [AnimatedTheme].
///
/// A switch of theme, density or accent fades over `motion.duration.fade` with
/// `motion.easing.standard`; under reduced motion the switch is immediate (duration zero), so it
/// is visible but never animated. The widget keeps no state: the caller passes a new
/// [DsContext], for example from [resolveDsContext].
///
/// ```dart
/// DsThemeScope(dsContext: ctx, child: const HomePage());
/// ```
class DsThemeScope extends StatelessWidget {
  /// Creates the scope for [dsContext] around [child].
  const DsThemeScope({super.key, required this.dsContext, required this.child});

  /// The inputs of the theme.
  final DsContext dsContext;

  /// The subtree that gets the theme.
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final reduced = dsContext.motion == DsMotionMode.reduced;
    return AnimatedTheme(
      data: buildDsTheme(dsContext),
      duration: reduced ? Duration.zero : DsMotion.full.durationFade,
      curve: DsMotion.full.easingStandard,
      child: child,
    );
  }
}
