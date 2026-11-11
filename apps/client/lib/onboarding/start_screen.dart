// SPDX-License-Identifier: Apache-2.0

/// The start screen: what shows until the app knows its first screen
/// (docs/decisions.d/AUD-client-onboarding-01.md).
library;

import 'package:flutter/widgets.dart';

import 'onboarding_card.dart';

/// The card of the onboarding with the mark of the product alone, while the app reads the
/// remembered address, the stored session and the local data. It has no field and no button,
/// so nothing of E1 can be used before the app decided between E1 and the main view, and no
/// animation, so it never keeps a frame scheduled. A screen reader hears [appName] for the
/// mark.
class StartScreen extends StatelessWidget {
  /// Creates the screen.
  const StartScreen({super.key, required this.appName});

  /// The name of the build (the instance and its product name are not known yet).
  final String appName;

  @override
  Widget build(BuildContext context) => OnboardingCard(
    children: [
      Semantics(label: appName, image: true, child: const BrandMark()),
    ],
  );
}
