// SPDX-License-Identifier: Apache-2.0

/// The device's choice for the code blocks of messages and the collapse limit of the
/// configuration, handed to the design system's renderer (01 6.14, R-PR-28; brief M4-49).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'providers/ports.dart';
import 'providers/session.dart';

part 'code_block_preference.g.dart';

/// Whether the code blocks of messages show one line per line with a horizontal scroll on this
/// device ("Wrap lines" off), from the local view state (`shell_view_state`, M4-49); false (long
/// lines wrap, 01 6.14 item 1) without a local database or until it answers. Kept per device
/// and never synchronized, like the other view state of the shell (D-M4-49-2).
@riverpod
Stream<bool> codeBlocksUnwrapped(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(false)
      : db.forumViewDao.watchCodeBlocksUnwrapped();
}

/// Stores on this device whether code blocks show [unwrapped]; [db] is the local database of
/// `localDatabaseProvider`, and nothing is stored without one (the toggle then switches only
/// its own block).
Future<void> storeCodeBlocksUnwrapped(
  LocalDatabase? db, {
  required bool unwrapped,
}) async {
  await db?.forumViewDao.setCodeBlocksUnwrapped(unwrapped: unwrapped);
}

/// Provides [DsCodeBlockScope] to [child]: whether long lines wrap comes from the device
/// ([codeBlocksUnwrappedProvider]), the toggle of any block stores the new choice, and the
/// collapse limit is `ui.code_block_collapse_lines` of the configuration.
///
/// The app puts it at its root, so every message renderer (history, thread, search, previews)
/// follows one choice per device.
///
/// ```dart
/// CodeBlockPreferenceScope(child: app);
/// ```
class CodeBlockPreferenceScope extends ConsumerWidget {
  /// Creates the scope around [child].
  const CodeBlockPreferenceScope({super.key, required this.child});

  /// The widgets below the scope.
  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final unwrapped = ref.watch(codeBlocksUnwrappedProvider).value ?? false;
    return DsCodeBlockScope(
      wrap: !unwrapped,
      collapseLines: ref.watch(appConfigProvider).codeBlockCollapseLines,
      onWrapChanged: (wrap) => storeCodeBlocksUnwrapped(
        ref.read(localDatabaseProvider),
        unwrapped: !wrap,
      ),
      child: child,
    );
  }
}
