// SPDX-License-Identifier: Apache-2.0

/// Running one administration change from a screen: busy state, the notice after it and the
/// failure through `adminFailure` (brief M4-26 Design notes 1 and 2).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/self_repository.dart';
import '../self/self_feedback.dart';
import 'problem_banner.dart';

/// The state of a screen that runs administration changes.
///
/// [runChange] runs one change (the confirmation came before it, the step-up comes from the
/// interceptor), shows its notice or its failure, and returns whether it succeeded; a refused
/// change (`403`) adds its key to [denied], so the screen hides that control (15 1 principle
/// 1). A cancelled step-up says nothing and changes nothing. A `412 version_conflict` (06 8)
/// with [runChange]'s `reload` reads the record again and says that it changed meanwhile, so
/// the next attempt sends the current version.
mixin AdminActionState<T extends StatefulWidget> on State<T> {
  /// Whether a change runs; the screen disables its buttons meanwhile.
  bool busy = false;

  /// The message of the last change, or null.
  SelfNotice? notice;

  /// Controls the server refused, by key.
  final Set<String> denied = {};

  /// Runs [change] under [key]; returns true when it succeeded. [reload] reads the record of
  /// the screen again after a `412 version_conflict`.
  Future<bool> runChange(
    String key,
    Future<SelfNotice?> Function() change, {
    Future<void> Function()? reload,
  }) async {
    final l = AppLocalizations.of(context);
    setState(() => busy = true);
    try {
      final result = await change();
      if (!mounted) return true;
      setState(() => notice = result);
      return true;
    } on SelfActionFailed catch (e) {
      if (reload != null && e.problem?.code == 'version_conflict') {
        await reload();
        if (mounted) {
          setState(
            () => notice = SelfNotice(
              l.adminVersionReloaded,
              tone: DsTone.warning,
            ),
          );
        }
        return false;
      }
      _fail(l, key, e);
      return false;
    } on Object catch (e) {
      _fail(l, key, e);
      return false;
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  void _fail(AppLocalizations l, String key, Object e) {
    final failure = adminFailure(l, e);
    if (mounted && failure != null) {
      setState(() {
        if (failure.denied) denied.add(key);
        notice = SelfNotice(failure.text, tone: DsTone.danger);
      });
    }
  }

  /// The banner of [notice], or nothing.
  Widget noticeBanner() => switch (notice) {
    final n? => selfNoticeBanner(n),
    null => const SizedBox.shrink(),
  };
}
