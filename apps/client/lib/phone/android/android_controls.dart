// SPDX-License-Identifier: Apache-2.0

/// The Android controls of the phone shell (docs/spec/01-product.md 5.4; brief M5-03 fixed
/// requirement 11 and Design note 6; docs/decisions.d/M5-03.md D-M5-03-6): system back pops the
/// stack of the tab, and a deeper page has the top app bar with the up arrow instead of the iOS
/// back button. The screens are the phone's of M5-02; only these controls differ.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

/// Lets system back reach the navigator of the active tab before it leaves the app.
///
/// While the tab's navigator ([navigator]) can pop (a page above the tab's first page, or a
/// sheet opened in it), system back pops it; the page that leaves goes back through the
/// shell's `onDidRemovePage`, so the router follows. On the first page of a tab nothing is
/// left to pop, and back reaches the app's router, which closes a sheet of the app's navigator
/// or leaves the app (`SystemNavigator.pop`), as Android expects at the root of an app. A
/// terminal session (M5-12) will intercept back below this scope to detach instead of ending.
///
/// Example:
/// ```dart
/// AndroidBackScope(navigator: key, child: Navigator(key: key, pages: pages));
/// ```
class AndroidBackScope extends StatelessWidget {
  /// Creates the scope around [child], which contains the navigator of [navigator].
  const AndroidBackScope({
    super.key,
    required this.navigator,
    required this.child,
  });

  /// The key of the tab's navigator.
  final GlobalKey<NavigatorState> navigator;

  /// The navigator.
  final Widget child;

  @override
  Widget build(BuildContext context) => NavigatorPopHandler<Object?>(
    onPopWithResult: (_) {
      final state = navigator.currentState;
      if (state != null) unawaited(state.maybePop());
    },
    child: child,
  );
}

/// The top app bar of a deeper page on Android (01 5.4): the up arrow, named after the page
/// below ([backLabel]), and the page's [title] when it has one (a thread names itself in its
/// own header below the bar).
class AndroidTopAppBar extends StatelessWidget {
  /// Creates the bar.
  const AndroidTopAppBar({
    super.key,
    required this.backLabel,
    required this.onBack,
    this.title,
  });

  /// The accessible text of the up arrow, for example "Back to Spaces".
  final String backLabel;

  /// Goes back.
  final VoidCallback onBack;

  /// The page's title, or null.
  final String? title;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final title = this.title;
    return AppBar(
      key: const ValueKey('phone.topAppBar'),
      // The shell's safe area already keeps the bar below the status bar.
      primary: false,
      automaticallyImplyLeading: false,
      backgroundColor: colors.surfaceCanvas,
      surfaceTintColor: colors.surfaceCanvas,
      elevation: 0,
      titleSpacing: 0,
      leading: DsTappable(
        key: const ValueKey('phone.back'),
        onTap: onBack,
        label: backLabel,
        excludeChildSemantics: true,
        child: Center(
          child: DsIcon(
            DsIcons.actionBack,
            size: DsIconSizeRole.md,
            color: colors.textPrimary,
            decorative: true,
          ),
        ),
      ),
      title: title == null
          ? null
          : Text(
              dsIsolate(title),
              overflow: TextOverflow.ellipsis,
              style: dsTextStyle(DsType.title)
                  .copyWith(color: colors.textPrimary),
            ),
    );
  }
}
