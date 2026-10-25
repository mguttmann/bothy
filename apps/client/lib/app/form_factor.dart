// SPDX-License-Identifier: Apache-2.0

/// Phone or desktop: which layout of the same screens the app shows (docs/spec/01-product.md
/// 5.1 and 5.3; brief M5-02 Design note 2; docs/decisions.d/M5-02.md D-M5-02-3).
library;

import 'package:flutter/widgets.dart';

/// The form factor of a build, decided by its platform, never by the window size: a desktop
/// window made narrow keeps the column layout of M4-18 (it stacks itself), a phone always
/// shows the navigation stack with the tab bar.
enum FormFactor {
  /// macOS and Windows: the columns of 01 5.1.
  desktop,

  /// iOS and Android: the navigation stack and the tab bar of 01 5.3 and 5.4.
  phone;

  /// The form factor of the `DevicePlatform` value [platform] (`ios`, `android`, `macos`,
  /// `windows`).
  ///
  /// Example: `FormFactor.ofPlatform('ios')` is [FormFactor.phone].
  static FormFactor ofPlatform(String platform) => switch (platform) {
    'ios' || 'android' => phone,
    _ => desktop,
  };

  /// Whether this is [phone].
  bool get isPhone => this == phone;
}

/// Hands the [FormFactor] of the app to widgets that read no provider (the onboarding card,
/// the screens of the golden tests); without a scope [of] gives [FormFactor.desktop].
class FormFactorScope extends InheritedWidget {
  /// Creates the scope.
  const FormFactorScope({
    super.key,
    required this.formFactor,
    required super.child,
  });

  /// The form factor below this scope.
  final FormFactor formFactor;

  /// The form factor of [context]: the nearest scope's, else [FormFactor.desktop].
  static FormFactor of(BuildContext context) =>
      context
          .dependOnInheritedWidgetOfExactType<FormFactorScope>()
          ?.formFactor ??
      FormFactor.desktop;

  @override
  bool updateShouldNotify(FormFactorScope oldWidget) =>
      oldWidget.formFactor != formFactor;
}
