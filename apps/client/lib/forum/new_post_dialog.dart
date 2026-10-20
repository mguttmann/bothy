// SPDX-License-Identifier: Apache-2.0

/// The dialog of a new forum post: title, first message and tags (brief M4-19 Design note 3;
/// a forum post needs its first message, `ThreadCreate`).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// What the person entered.
final class NewPost {
  /// Creates the input.
  const NewPost({
    required this.title,
    required this.contentMd,
    required this.tagIds,
  });

  /// The title.
  final String title;

  /// The first message (Markdown).
  final String contentMd;

  /// The chosen tags.
  final List<ForumTagId> tagIds;
}

/// Shows the dialog over [context] with the [tags] of the forum; returns the input, or null
/// when the person cancels.
/// [initial] fills the dialog with a post to change (a refused one).
Future<NewPost?> showNewPostDialog(
  BuildContext context,
  List<ForumTag> tags, {
  NewPost? initial,
}) => showDialog<NewPost>(
  context: context,
  builder: (context) => NewPostDialog(tags: tags, initial: initial),
);

/// The dialog; "Create post" is enabled once title and first message are not blank. Every tag
/// of the forum is offered, a restricted one too: whether the person may set it (`manage`) is
/// the server's decision, not the client's (15 1 principle 1); a refused post stays in the
/// outbox and the post list shows it with "Try again" and "Discard".
class NewPostDialog extends StatefulWidget {
  /// Creates the dialog.
  const NewPostDialog({super.key, required this.tags, this.initial});

  /// The tags of the forum.
  final List<ForumTag> tags;

  /// The post the dialog starts with, or null for an empty one.
  final NewPost? initial;

  @override
  State<NewPostDialog> createState() => _NewPostDialogState();
}

class _NewPostDialogState extends State<NewPostDialog> {
  late final TextEditingController _title = TextEditingController(
    text: widget.initial?.title,
  );
  late final TextEditingController _message = TextEditingController(
    text: widget.initial?.contentMd,
  );
  late final Set<ForumTagId> _tags = {...?widget.initial?.tagIds};

  @override
  void initState() {
    super.initState();
    _title.addListener(_changed);
    _message.addListener(_changed);
  }

  void _changed() => setState(() {});

  @override
  void dispose() {
    _title.dispose();
    _message.dispose();
    super.dispose();
  }

  bool get _ready =>
      _title.text.trim().isNotEmpty && _message.text.trim().isNotEmpty;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return AlertDialog(
      key: const ValueKey('newPost.dialog'),
      title: Text(l.postListNewPost),
      content: SizedBox(
        width: DsLayout.listWidth * 1.5,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                key: const ValueKey('newPost.title'),
                controller: _title,
                autofocus: true,
                decoration: InputDecoration(labelText: l.newPostTitle),
              ),
              const SizedBox(height: DsSpace.s12),
              TextField(
                key: const ValueKey('newPost.message'),
                controller: _message,
                minLines: 3,
                maxLines: 8,
                decoration: InputDecoration(labelText: l.newPostMessage),
              ),
              if (widget.tags.isNotEmpty) ...[
                const SizedBox(height: DsSpace.s12),
                Text(l.newPostTags, style: dsTextStyle(DsType.label)),
                const SizedBox(height: DsSpace.s6),
                Wrap(
                  spacing: DsSpace.s6,
                  runSpacing: DsSpace.s6,
                  children: [
                    for (final t in widget.tags)
                      FilterChip(
                        key: ValueKey('newPost.tag.${t.id.value}'),
                        label: Text(dsIsolate(t.name)),
                        selected: _tags.contains(t.id),
                        onSelected: (on) => setState(
                          () => on ? _tags.add(t.id) : _tags.remove(t.id),
                        ),
                      ),
                  ],
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        DsButton(
          key: const ValueKey('newPost.cancel'),
          label: l.newPostCancel,
          style: DsButtonStyle.ghost,
          onPressed: () => Navigator.of(context).pop(),
        ),
        DsButton(
          key: const ValueKey('newPost.create'),
          label: l.newPostCreate,
          style: DsButtonStyle.primary,
          onPressed: _ready
              ? () => Navigator.of(context).pop(
                  NewPost(
                    title: _title.text.trim(),
                    contentMd: _message.text.trim(),
                    tagIds: [
                      for (final t in widget.tags)
                        if (_tags.contains(t.id)) t.id,
                    ],
                  ),
                )
              : null,
        ),
      ],
    );
  }
}
