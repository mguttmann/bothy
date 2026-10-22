// SPDX-License-Identifier: Apache-2.0

/// Screen reader access to the terminal (docs/spec/01-product.md 8, docs/design/components.md
/// 12 "Semantics"): the line at the cursor is the label of the screen, the visible screen is
/// read on command, and running output is never read aloud (no live region).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/widgets.dart';
import 'package:terminal_ui/src/render/terminal_screen.dart';

/// Wraps the renderer in one semantics node whose label is the line at the cursor and whose
/// custom action "Read screen" announces the visible lines.
///
/// The label follows the screen on every rebuild, but the node is not a live region, so a
/// screen reader reads it when the person moves there, not while a program prints.
///
/// ```dart
/// TerminalScreenSemantics(screen: controller.screen, child: view)
/// ```
class TerminalScreenSemantics extends StatelessWidget {
  /// Creates the wrapper over [child] for [screen].
  const TerminalScreenSemantics({
    super.key,
    required this.screen,
    required this.child,
  });

  /// The screen whose cursor line and visible lines are read.
  final TerminalScreen screen;

  /// The renderer.
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    return ListenableBuilder(
      listenable: screen,
      builder: (context, child) => Semantics(
        container: true,
        liveRegion: false,
        label: l.terminalScreenLabel(screen.cursorLine()),
        customSemanticsActions: {
          CustomSemanticsAction(label: l.terminalReadScreen): () =>
              readScreen(context, screen),
        },
        child: ExcludeSemantics(child: child!),
      ),
      child: child,
    );
  }
}

/// Announces the visible lines of [screen] once (the command "Read screen").
void readScreen(BuildContext context, TerminalScreen screen) {
  final text = screen.visibleText();
  if (text.isEmpty) return;
  SemanticsService.sendAnnouncement(
    View.of(context),
    text,
    Directionality.of(context),
  );
}
