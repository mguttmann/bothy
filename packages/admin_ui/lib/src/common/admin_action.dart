// SPDX-License-Identifier: Apache-2.0

/// Running one administration change from a screen: busy state, the notice after it and the
/// failure through `adminFailure` (brief M4-26 Design notes 1 and 2).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../self/self_feedback.dart';
import 'problem_banner.dart';

/// The state of a screen that runs administration changes.
///
/// [runChange] runs one change (the confirmation came before it, the step-up comes from the
/// interceptor), shows its notice or its failure, and returns whether it succeeded; a refused
/// change (`403`) adds its key to [denied], so the screen hides that control (15 1 principle
/// 1). A cancelled step-up says nothing and changes nothing.
mixin AdminActionState<T extends StatefulWidget> on State<T> {
  /// Whether a change runs; the screen disables its buttons meanwhile.
  bool busy = false;

  /// The message of the last change, or null.
  SelfNotice? notice;

  /// Controls the server refused, by key.
  final Set<String> denied = {};

  /// Runs [change] under [key]; returns true when it succeeded.
  Future<bool> runChange(
    String key,
    Future<SelfNotice?> Function() change,
  ) async {
    final l = AppLocalizations.of(context);
    setState(() => busy = true);
    try {
      final result = await change();
      if (!mounted) return true;
      setState(() => notice = result);
      return true;
    } on Object catch (e) {
      final failure = adminFailure(l, e);
      if (mounted && failure != null) {
        setState(() {
          if (failure.denied) denied.add(key);
          notice = SelfNotice(failure.text, tone: DsTone.danger);
        });
      }
      return false;
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  /// The banner of [notice], or nothing.
  Widget noticeBanner() => switch (notice) {
    final n? => selfNoticeBanner(n),
    null => const SizedBox.shrink(),
  };
}
