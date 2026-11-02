// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';

/// Closes the on-screen keyboard as iOS and Android do, for every text field of the app at one
/// place (FIX-keyboard-dismiss-focus-ring).
///
/// - A tap anywhere outside the focused text field unfocuses it, which hides the keyboard; the
///   text stays. Flutter does this on the desktop and the web, but not for a touch on iOS and
///   Android, so a person could only close the keyboard by leaving the screen. A control that
///   belongs to the field (the composer's attach, send and voice buttons and its suggestion
///   list) sits in a [TextFieldTapRegion], so a tap on it works with the keyboard open.
/// - On iOS and Android, dragging a scroll view (the message list, a settings page) hides the
///   keyboard ([ScrollViewKeyboardDismissBehavior.onDrag]). The desktop keeps the focus while
///   a trackpad scrolls a list.
///
/// Install it once around the app's navigator, for example in `MaterialApp.builder`, so it
/// also covers dialogs and sheets:
///
/// ```dart
/// builder: (context, child) => DsKeyboardDismiss(child: child!),
/// ```
class DsKeyboardDismiss extends StatelessWidget {
  /// Applies the behavior to [child].
  const DsKeyboardDismiss({super.key, required this.child});

  /// The app.
  final Widget child;

  static bool get _touchPlatform =>
      defaultTargetPlatform == TargetPlatform.iOS ||
      defaultTargetPlatform == TargetPlatform.android;

  @override
  Widget build(BuildContext context) {
    Widget content = child;
    if (_touchPlatform) {
      content = ScrollConfiguration(
        behavior: ScrollConfiguration.of(context).copyWith(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        ),
        child: content,
      );
    }
    return Actions(
      actions: <Type, Action<Intent>>{
        EditableTextTapOutsideIntent: _UnfocusOnTapOutside(),
      },
      child: content,
    );
  }
}

/// Unfocuses the text field on every tap outside it, also a touch on iOS and Android.
class _UnfocusOnTapOutside extends ContextAction<EditableTextTapOutsideIntent> {
  @override
  void invoke(EditableTextTapOutsideIntent intent, [BuildContext? context]) {
    intent.focusNode.unfocus();
  }
}
