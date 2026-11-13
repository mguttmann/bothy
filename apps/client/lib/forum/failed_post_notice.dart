// SPDX-License-Identifier: Apache-2.0

/// The notice of a new post the server refused, or whose upload failed for good (04 13.2 item
/// 5: nothing is dropped silently; FEAT-NEW-CHAT-ATTACH, FNCA/4): its title and reason with
/// "Edit", "Try again" and "Discard" at the top of the post list and in the view of the sent
/// post.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../files/attachment_mapping.dart' show uploadFailureText;

/// A `createThread` of the outbox that cannot go without the person: its entry ID, what the
/// person typed and why the server refused it, or why one of its uploads failed.
final class FailedPost {
  /// Creates the item.
  const FailedPost({
    required this.entryId,
    required this.title,
    this.contentMd = '',
    this.tagIds = const [],
    this.code,
    this.reason,
    this.uploadFailed = false,
    this.hasFiles = false,
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

  /// Whether [code] and [reason] are the failure of one of its uploads: the post itself was not
  /// sent yet.
  final bool uploadFailed;

  /// Whether the first message has files; "Edit" is not offered then, because the files
  /// cannot go back into the composer (the upload deleted their copies; FNCA/4).
  final bool hasFiles;
}

/// The notice of [post]; [onRetry] sends it again, [onDiscard] deletes it, [onEdit] (null:
/// not offered) moves it back into the new chat view.
class FailedPostNotice extends StatelessWidget {
  /// Creates the notice.
  const FailedPostNotice({
    super.key,
    required this.post,
    required this.onRetry,
    required this.onDiscard,
    this.onEdit,
  });

  /// The refused post.
  final FailedPost post;

  /// Sends it again.
  final VoidCallback onRetry;

  /// Deletes it.
  final VoidCallback onDiscard;

  /// Opens it in the new chat view to change and send it again (04 13.2 item 5), or null.
  final VoidCallback? onEdit;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final t = dsToneColors(context, DsTone.danger);
    final code = post.code;
    final reason = post.uploadFailed && code != null
        ? uploadFailureText(l, code, reason: post.reason)
        : post.reason ?? dsReasonText(l, code) ?? code;
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
              if (onEdit case final edit?)
                DsButton(
                  key: ValueKey('postList.failed.edit.${post.entryId}'),
                  label: l.postListEdit,
                  style: DsButtonStyle.secondary,
                  onPressed: edit,
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
