// SPDX-License-Identifier: Apache-2.0

/// The route `/share` of the app on Android: the target sheet of the share the runner
/// received (brief M5-06 Design note 6; 01 5.4).
library;

import 'dart:async';
import 'dart:io';

import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../files/providers.dart';
import '../thread/providers.dart';
import 'providers.dart';
import 'share_android.dart';
import 'share_outbox.dart';
import 'share_sheet.dart';
import 'share_target.dart';
import 'share_targets.dart';

/// The page of `/share`: [ShareSheet] over the pending share.
///
/// The recent targets come from the local database of the signed-in instance, the search from
/// its channels and `searchThreadTitles`; the send goes through the outbox ([ShareOutbox]) and
/// then shows the target: the thread, or the forum of a new post. Cancel deletes the cache
/// copies and goes home. Without a pending share (the app was restored on this path) the page
/// goes home at once.
class ShareRoute extends ConsumerStatefulWidget {
  /// Creates the page.
  const ShareRoute({super.key});

  @override
  ConsumerState<ShareRoute> createState() => _ShareRouteState();
}

class _ShareRouteState extends ConsumerState<ShareRoute> {
  SharedContent? _content;
  ShareSheetController? _sheet;

  @override
  void dispose() {
    _sheet?.dispose();
    super.dispose();
  }

  ShareSheetController _controllerOf(SharedContent content) {
    final services = ref.read(signedInServicesProvider);
    final db = services?.database;
    final source = db == null || services == null
        ? null
        : ShareTargetSource(db, loadThread: threadLoaderOf(services));
    final search = source == null || services == null
        ? null
        : ShareTargetSearch(
            source: source,
            titles: ApiThreadTitleSearch(services.auth.api),
          );
    final max = ref.read(appConfigProvider).shareRecentTargets;
    return ShareSheetController(
      files: content.files,
      text: content.text ?? '',
      limits: ref.read(instanceLimitsProvider).value,
      recent: () async {
        final me = services?.engine.catchUp.me;
        if (source == null || me == null) return const <ShareTarget>[];
        return source.recent(me: me, max: max);
      },
      search: search == null
          ? (_) async => const <ShareTarget>[]
          : search.search,
    );
  }

  Future<void> _send(SharedContent content, ShareChoice choice) async {
    final services = ref.read(signedInServicesProvider);
    if (services == null) return;
    final sender = SharedContentSender(
      outbox: ShareOutbox(
        outbox: services.engine.outbox,
        entries: services.database.outboxDao,
        outgoing: () async => ref.read(outgoingFolderProvider),
      ),
      newId: services.ids.next,
    );
    try {
      await sender.send(
        content,
        target: choice.target,
        comment: choice.comment,
        title: choice.title,
      );
    } on FileSystemException {
      // A copy failed: nothing was written, the sheet stays and the person tries again.
      return;
    }
    if (!mounted) return;
    ref.read(pendingShareProvider.notifier).done(content);
    final target = choice.target;
    final thread = target.threadId;
    GoRouter.of(context).go(
      thread == null ? '/c/${target.channelId.value}' : '/t/${thread.value}',
    );
  }

  void _cancel(SharedContent content) {
    unawaited(AndroidShare.discard(content));
    ref.read(pendingShareProvider.notifier).done(content);
    GoRouter.of(context).go('/');
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(instanceLimitsProvider, (_, next) {
      final sheet = _sheet;
      if (sheet != null) sheet.limits = next.value;
    });
    final content = ref.watch(pendingShareProvider);
    if (content == null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && ref.read(pendingShareProvider) == null) {
          GoRouter.of(context).go('/');
        }
      });
      return const SizedBox.shrink();
    }
    if (!identical(content, _content)) {
      // The sheet of the earlier share unmounts in this frame; its state goes after it.
      final old = _sheet;
      if (old != null) {
        WidgetsBinding.instance.addPostFrameCallback((_) => old.dispose());
      }
      _sheet = _controllerOf(content);
      _content = content;
    }
    return ShareSheet(
      key: ValueKey('share.${content.id}'),
      controller: _sheet!,
      onCancel: () => _cancel(content),
      onSend: (choice) => _send(content, choice),
    );
  }
}
