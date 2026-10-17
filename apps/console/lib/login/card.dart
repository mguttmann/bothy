// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

/// The card of the onboarding screens in the center of the empty page (01 5.6, desktop form):
/// scrollable, so text scaling 2 never cuts it off; focus follows the reading order.
class ConsoleCard extends StatelessWidget {
  /// Creates the card around [children].
  const ConsoleCard({super.key, required this.children});

  /// The content, top to bottom.
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    // A Material, so the text of the card takes the colors of the theme also outside a Scaffold.
    return Material(
      color: colors.surfaceCanvas,
      child: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(DsSpace.s24),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: DsLayout.listWidth),
            child: FocusTraversalGroup(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: children,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// A heading of a card, marked as a header for screen readers.
class CardTitle extends StatelessWidget {
  /// Creates the title.
  const CardTitle(this.text, {super.key});

  /// The text.
  final String text;

  @override
  Widget build(BuildContext context) => Semantics(
    header: true,
    child: Text(
      text,
      style: dsTextStyle(DsType.title),
      textAlign: TextAlign.center,
    ),
  );
}

/// The gap between the parts of a card.
const Widget cardGap = SizedBox(height: DsSpace.s16);
