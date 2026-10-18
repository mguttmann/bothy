// SPDX-License-Identifier: Apache-2.0

/// The notice of a new post the server refused (04 13.2 item 5: nothing is dropped silently):
/// its title and reason with "Edit", "Try again" and "Discard" at the top of the post list.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// A refused `createThread` of the outbox: its entry ID, what the person typed and why the
/// server refused it.
final class FailedPost {
  /// Creates the item.
  const FailedPost({
    required this.entryId,
    required this.title,
    this.contentMd = '',
    this.tagIds = const [],
    this.code,
    this.reason,
  });

  /// The outbox entry.
  final String entryId;

  /// The title of the post.
  final String title;

  /// The first message.
  final String contentMd;

  /// The chosen tags.
  final List<ForumTagId> tagIds;

  /// The problem code of the refusal, or null.
  final String? code;

  /// The server's reason of the refusal, or null.
  final String? reason;
}

/// The notice of [post]; [onRetry] sends it again, [onDiscard] deletes it.
class FailedPostNotice extends StatelessWidget {
  /// Creates the notice.
  const FailedPostNotice({
    super.key,
    required this.post,
    required this.onRetry,
    required this.onDiscard,
    required this.onEdit,
  });

  /// The refused post.
  final FailedPost post;

  /// Sends it again.
  final VoidCallback onRetry;

  /// Deletes it.
  final VoidCallback onDiscard;

  /// Opens it in the "New post" dialog to change and send it again (04 13.2 item 5).
  final VoidCallback onEdit;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final t = dsToneColors(context, DsTone.danger);
    final reason = post.reason ?? dsReasonText(l, post.code) ?? post.code;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
      child: Semantics(
        container: true,
        liveRegion: true,
        child: Container(
          padding: const EdgeInsets.all(DsSpace.s8),
          decoration: BoxDecoration(
            color: t.subtle,
            border: Border.all(color: t.border, width: DsBorderWidth.hairline),
            borderRadius: BorderRadius.circular(DsRadius.md),
          ),
          child: Wrap(
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s6,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Text(
                l.postListCreateFailed(dsIsolate(post.title)),
                style: dsTextStyle(DsType.bodySm).copyWith(color: t.fg),
              ),
              if (reason != null)
                Text(
                  l.postListFailureReason(reason),
                  style: dsTextStyle(DsType.caption).copyWith(color: t.fg),
                ),
              DsButton(
                key: ValueKey('postList.failed.edit.${post.entryId}'),
                label: l.postListEdit,
                style: DsButtonStyle.secondary,
                onPressed: onEdit,
              ),
              DsButton(
                key: ValueKey('postList.failed.retry.${post.entryId}'),
                label: l.postListTryAgain,
                style: DsButtonStyle.secondary,
                onPressed: onRetry,
              ),
              DsButton(
                key: ValueKey('postList.failed.discard.${post.entryId}'),
                label: l.postListDiscard,
                style: DsButtonStyle.ghost,
                onPressed: onDiscard,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
