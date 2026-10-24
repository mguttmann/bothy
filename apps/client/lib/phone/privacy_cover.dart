// SPDX-License-Identifier: Apache-2.0

/// The privacy cover of the phone (docs/spec/01-product.md 5.6 E6: "The content is hidden,
/// also in the app switcher"; brief M5-02 fixed requirement 14 and Design note 6).
library;

import 'dart:typed_data';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../onboarding/onboarding_card.dart' show BrandMark;
import '../platform/lifecycle.dart';

/// Lays the cover over [child] while [cover] says so.
///
/// The content stays in the tree (the router, drafts and scroll positions survive) but is
/// offstage: it is neither painted, so the app switcher picture shows only the cover, nor in
/// the semantics tree, nor hit-testable. The cover shows the logo and `branding.product_name`
/// of the instance ([productName]), never content.
class PrivacyCoverHost extends StatelessWidget {
  /// Creates the host.
  const PrivacyCoverHost({
    super.key,
    required this.cover,
    required this.productName,
    required this.child,
    this.logo,
  });

  /// The cover state.
  final PrivacyCover cover;

  /// `branding.product_name`, or the build's app name before discovery.
  final String productName;

  /// The logo bytes, or null.
  final Uint8List? logo;

  /// The app.
  final Widget child;

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: cover,
    builder: (context, _) => Stack(
      fit: StackFit.expand,
      children: [
        Offstage(offstage: cover.covered, child: child),
        if (cover.covered)
          PrivacyCoverView(productName: productName, logo: logo),
      ],
    ),
  );
}

/// The full-screen surface of the cover: the logo and the product name, nothing else.
class PrivacyCoverView extends StatelessWidget {
  /// Creates the view.
  const PrivacyCoverView({super.key, required this.productName, this.logo});

  /// `branding.product_name`.
  final String productName;

  /// The logo bytes, or null.
  final Uint8List? logo;

  @override
  Widget build(BuildContext context) => Material(
    key: const ValueKey('privacy.cover'),
    color: context.dsColors.surfaceCanvas,
    child: Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          BrandMark(logo: logo),
          const SizedBox(height: DsSpace.s16),
          Text(
            productName,
            textAlign: TextAlign.center,
            style: dsTextStyle(DsType.title)
                .copyWith(color: context.dsColors.textPrimary),
          ),
        ],
      ),
    ),
  );
}
