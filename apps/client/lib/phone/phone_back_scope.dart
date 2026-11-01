// SPDX-License-Identifier: Apache-2.0

/// The back action of a phone page that draws it in its own header (brief M5-17; 01 5.3 M3):
/// a thread's one-line header starts with the back button, so the page frame shows no back bar
/// above it. While the page shows no such header (the thread is still loading, or it is not
/// found), the frame keeps its back bar, so a page is never without a way back.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The number of [PhoneHeaderBack] buttons showing the back action of one page frame; the
/// frame shows its own back bar while [count] is 0. A change after [dispose] (a button that
/// leaves in the same frame as its page) is ignored.
class PhoneBackClaims extends ChangeNotifier {
  int _count = 0;
  bool _disposed = false;

  /// The buttons showing the action now.
  int get count => _count;

  /// One more ([by] 1) or one less ([by] -1) button.
  void change(int by) {
    if (_disposed) return;
    _count += by;
    notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }
}

/// Hands the back action of the page frame to the page below it, which shows it in its
/// header ([PhoneHeaderBack]); [claims] counts the header buttons that show it now.
class PhoneBackScope extends InheritedWidget {
  /// Creates the scope; [label] names the page the button goes back to.
  const PhoneBackScope({
    super.key,
    required this.label,
    required this.onBack,
    this.claims,
    required super.child,
  });

  /// The name of the page below, for the label "Back to {label}".
  final String label;

  /// Goes back.
  final VoidCallback onBack;

  /// The buttons below showing the action; null where nobody listens (a test or a golden
  /// scene).
  final PhoneBackClaims? claims;

  /// The nearest scope of [context], or null where the page has a back bar of its own or
  /// none (the desktop, a tab's first page).
  static PhoneBackScope? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<PhoneBackScope>();

  @override
  bool updateShouldNotify(PhoneBackScope oldWidget) =>
      oldWidget.label != label ||
      oldWidget.onBack != onBack ||
      oldWidget.claims != claims;
}

/// The back button at the start of a one-line page header: the back icon with the hit area
/// `density.target-min` and the label "Back to {label}"; nothing without a [PhoneBackScope].
/// While it is in the tree it claims the action of the scope, so the frame hides its bar.
class PhoneHeaderBack extends StatefulWidget {
  /// Creates the button.
  const PhoneHeaderBack({super.key});

  @override
  State<PhoneHeaderBack> createState() => _PhoneHeaderBackState();
}

class _PhoneHeaderBackState extends State<PhoneHeaderBack> {
  /// The claims of the scope this button belongs to now.
  PhoneBackClaims? _wanted;

  /// The claims this button has counted itself in.
  PhoneBackClaims? _counted;

  /// Brings [_counted] to [_wanted] after the frame: the frame rebuilds from the count, which
  /// must never change during a build.
  void _sync() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (identical(_counted, _wanted)) return;
      _counted?.change(-1);
      _counted = _wanted;
      _counted?.change(1);
    });
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _wanted = PhoneBackScope.maybeOf(context)?.claims;
    _sync();
  }

  @override
  void dispose() {
    _wanted = null;
    _sync();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scope = PhoneBackScope.maybeOf(context);
    if (scope == null) return const SizedBox.shrink();
    final l = AppLocalizations.of(context);
    final target = context.dsDensity.targetMin;
    return DsTappable(
      key: const ValueKey('phone.back'),
      onTap: scope.onBack,
      label: l.phoneBackTo(scope.label),
      excludeChildSemantics: true,
      child: SizedBox.square(
        dimension: target,
        child: Center(
          child: DsIcon(
            DsIcons.actionBack,
            size: DsIconSizeRole.md,
            color: context.dsAccent.textLink,
            decorative: true,
          ),
        ),
      ),
    );
  }
}
