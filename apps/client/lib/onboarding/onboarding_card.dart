// SPDX-License-Identifier: Apache-2.0

/// The card the onboarding screens share on the desktop (docs/spec/01-product.md 5.6: a card
/// in the center of an empty window with the branding of the instance).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// The card: scrollable (text scaling 2 never cuts it off), focus in reading order, Enter
/// triggers [onEnter] and Esc [onEscape] (01 5.6 "Keyboard").
class OnboardingCard extends StatelessWidget {
  /// Creates the card around [children].
  const OnboardingCard({
    super.key,
    required this.children,
    this.onEnter,
    this.onEscape,
  });

  /// The content, top to bottom.
  final List<Widget> children;

  /// The main action of the screen, or null.
  final VoidCallback? onEnter;

  /// One step back, or null.
  final VoidCallback? onEscape;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return Focus(
      autofocus: true,
      // Enter runs the main action only from the card itself or from a text field; a focused
      // button or radio keeps Enter for its own activation (01 5.6 "Keyboard").
      onKeyEvent: (node, event) {
        if (event is! KeyDownEvent) return KeyEventResult.ignored;
        final key = event.logicalKey;
        if (key == LogicalKeyboardKey.escape && onEscape != null) {
          onEscape!();
          return KeyEventResult.handled;
        }
        if ((key == LogicalKeyboardKey.enter ||
                key == LogicalKeyboardKey.numpadEnter) &&
            onEnter != null &&
            _enterIsMine(node)) {
          onEnter!();
          return KeyEventResult.handled;
        }
        return KeyEventResult.ignored;
      },
      child: Material(
        color: colors.surfaceCanvas,
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(DsSpace.s24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: DsLayout.overlayWidth,
              ),
              // A Material, so list tiles and ink of the card paint on the card itself.
              child: Material(
                color: colors.surfaceRaised,
                shape: RoundedRectangleBorder(
                  side: BorderSide(
                    color: colors.borderSubtle,
                    width: DsBorderWidth.hairline,
                  ),
                  borderRadius: BorderRadius.circular(DsRadius.lg),
                ),
                child: Padding(
                  padding: const EdgeInsets.all(DsSpace.s24),
                  child: FocusTraversalGroup(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      mainAxisSize: MainAxisSize.min,
                      children: children,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  /// Whether Enter belongs to the card: the card itself or a text field has the focus.
  static bool _enterIsMine(FocusNode card) {
    final focus = FocusManager.instance.primaryFocus;
    if (focus == null || identical(focus, card)) return true;
    return focus.context?.findAncestorWidgetOfExactType<EditableText>() != null;
  }
}

/// The logo of the instance (served by the server through `getBrandingLogo`), or the mark in
/// the accent color while the logo is missing (01 12).
class BrandMark extends StatelessWidget {
  /// Creates the mark.
  const BrandMark({super.key, this.logo});

  /// The logo bytes, or null.
  final Uint8List? logo;

  @override
  Widget build(BuildContext context) {
    final bytes = logo;
    return Center(
      child: SizedBox.square(
        dimension: DsSize.avatarLg,
        child: bytes != null
            ? Image.memory(
                bytes,
                fit: BoxFit.contain,
                excludeFromSemantics: true,
              )
            : DecoratedBox(
                decoration: BoxDecoration(
                  color: context.dsAccent.accentBg,
                  borderRadius: BorderRadius.circular(DsRadius.md),
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
      style: dsTextStyle(DsType.title)
          .copyWith(color: context.dsColors.textPrimary),
      textAlign: TextAlign.center,
    ),
  );
}

/// A body text of a card.
class CardText extends StatelessWidget {
  /// Creates the text.
  const CardText(this.text, {super.key, this.center = false});

  /// The text.
  final String text;

  /// Whether the text is centered.
  final bool center;

  @override
  Widget build(BuildContext context) => Text(
    text,
    style: dsTextStyle(DsType.body)
        .copyWith(color: context.dsColors.textSecondary),
    textAlign: center ? TextAlign.center : TextAlign.start,
  );
}

/// The gap between the parts of a card.
const Widget cardGap = SizedBox(height: DsSpace.s16);

/// The small gap between two controls.
const Widget controlGap = SizedBox(height: DsSpace.s8);
