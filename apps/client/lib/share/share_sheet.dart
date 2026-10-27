// SPDX-License-Identifier: Apache-2.0

/// The target sheet of a share in Flutter, used on Android (brief M5-06 Design notes 1 and 6;
/// docs/spec/01-product.md 5.3 M6 and 5.4): a header with Cancel, "Share to" and Send, the
/// preview of the first item, the comment, the target list with its search and the note that
/// only targets with write permission are offered.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../files/attach_controller.dart'
    show AttachRefusal, FileTooLarge, TooManyFiles;
import '../files/attachment_mapping.dart';
import 'share_target.dart';

/// What the person chose in the sheet.
final class ShareChoice {
  /// Creates the choice.
  const ShareChoice({required this.target, required this.comment, this.title});

  /// The target.
  final ShareTarget target;

  /// The comment, the text of the message (may be empty when files are shared).
  final String comment;

  /// The title of a new post; null for the other kinds.
  final String? title;
}

/// The state of the sheet: the shared files, the recent targets, the search, the chosen target
/// and the limits.
///
/// The recent targets come from the local database (newest first, at most
/// `client.share.recent_targets`), the search from `ShareTargetSearch`; both offer only targets
/// whose channel the server reports as writable. The limits are `file_max_upload_bytes` and
/// `message_max_attachments` of the instance; a file over one keeps Send disabled with the
/// reason ([refusal]); null limits leave the check to the server.
///
/// Example:
/// ```dart
/// final sheet = ShareSheetController(files: content.files, text: content.text ?? '',
///     limits: limits, recent: () => source.recent(me: me, max: 8), search: search.search);
/// ```
final class ShareSheetController extends ChangeNotifier {
  /// Creates the state; [recent] loads the recent targets once, [search] answers a query.
  ShareSheetController({
    required this.files,
    required Future<List<ShareTarget>> Function() recent,
    required Future<List<ShareTarget>> Function(String query) search,
    this.text = '',
    InstanceLimits? limits,
  }) : _limits = limits, // ignore: prefer_initializing_formals
       _loadRecent = recent,
       _search = search; // ignore: prefer_initializing_formals

  /// The shared files in their order.
  final List<ShareFile> files;

  /// The shared text or link, the start of the comment.
  final String text;

  /// The limits of the instance, or null while unknown; they may arrive after the sheet
  /// opened.
  InstanceLimits? get limits => _limits;
  set limits(InstanceLimits? value) {
    if (value == _limits) return;
    _limits = value;
    _changed();
  }

  InstanceLimits? _limits;

  final Future<List<ShareTarget>> Function() _loadRecent;
  final Future<List<ShareTarget>> Function(String query) _search;
  Future<void>? _loading;
  List<ShareTarget> _recent = const [];
  List<ShareTarget>? _results;
  ShareTarget? _selected;
  int _generation = 0;
  bool _sending = false;
  bool _disposed = false;

  /// The targets shown: the search results while a query is typed, else the recent ones.
  List<ShareTarget> get targets => _results ?? _recent;

  /// The chosen target, or null.
  ShareTarget? get selected => _selected;

  /// Whether a send is running.
  bool get sending => _sending;

  /// Why the files cannot be sent, or null: more files than `message_max_attachments`, or
  /// the first file over `file_max_upload_bytes`.
  AttachRefusal? get refusal {
    final maxCount = limits?.messageMaxAttachments;
    if (maxCount != null && files.length > maxCount) {
      return TooManyFiles(maxCount);
    }
    final maxBytes = limits?.fileMaxUploadBytes;
    if (maxBytes == null) return null;
    for (final f in files) {
      if (f.sizeBytes > maxBytes) return FileTooLarge(f.name, maxBytes);
    }
    return null;
  }

  /// Loads the recent targets once; a failure leaves the list empty.
  Future<void> load() => _loading ??= () async {
    try {
      _recent = await _loadRecent();
    } on Object {
      _recent = const [];
    }
    _changed();
  }();

  /// Chooses [target].
  void select(ShareTarget target) {
    _selected = target;
    _changed();
  }

  /// Searches [query]; a blank query shows the recent targets again. An answer to an older
  /// query is dropped, a failing search finds nothing.
  Future<void> setQuery(String query) async {
    final generation = ++_generation;
    if (query.trim().isEmpty) {
      _results = null;
      _changed();
      return;
    }
    List<ShareTarget> found;
    try {
      found = await _search(query.trim());
    } on Object {
      found = const [];
    }
    if (generation != _generation) return;
    _results = found;
    _changed();
  }

  /// Whether Send is enabled with [comment] and [title]: a target is chosen, the files fit
  /// the limits, there is text or a file, a new post has a title, and no send runs.
  bool canSend({required String comment, required String title}) {
    final target = _selected;
    return target != null &&
        !_sending &&
        refusal == null &&
        (files.isNotEmpty || comment.trim().isNotEmpty) &&
        (target.kind != ShareTargetKind.newPost || title.trim().isNotEmpty);
  }

  /// Hands the choice to [onSend] once; a second call while it runs does nothing.
  Future<void> send(
    Future<void> Function(ShareChoice choice) onSend, {
    required String comment,
    required String title,
  }) async {
    final target = _selected;
    if (target == null || !canSend(comment: comment, title: title)) return;
    _sending = true;
    _changed();
    try {
      await onSend(
        ShareChoice(
          target: target,
          comment: comment.trim(),
          title: target.kind == ShareTargetKind.newPost ? title.trim() : null,
        ),
      );
    } finally {
      _sending = false;
      _changed();
    }
  }

  void _changed() {
    if (!_disposed) notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }
}

/// The sheet over [controller]; [onCancel] closes it, [onSend] sends the choice.
///
/// Every interactive element has a stable key for the device tests: `share.cancel`,
/// `share.send`, `share.comment`, `share.title`, `share.search` and `share.target.<index>`
/// (the index in the list shown). The list shows only the display paths of the targets,
/// never the content of a thread.
class ShareSheet extends StatefulWidget {
  /// Creates the sheet.
  const ShareSheet({
    super.key,
    required this.controller,
    required this.onCancel,
    required this.onSend,
  });

  /// The state.
  final ShareSheetController controller;

  /// Closes the sheet without sending.
  final VoidCallback onCancel;

  /// Sends the choice through the app's outbox.
  final Future<void> Function(ShareChoice choice) onSend;

  @override
  State<ShareSheet> createState() => _ShareSheetState();
}

class _ShareSheetState extends State<ShareSheet> {
  late final TextEditingController _comment = TextEditingController(
    text: widget.controller.text,
  );
  final TextEditingController _title = TextEditingController();
  final TextEditingController _query = TextEditingController();

  ShareSheetController get _sheet => widget.controller;

  @override
  void initState() {
    super.initState();
    _comment.addListener(_rebuild);
    _title.addListener(_rebuild);
    unawaited(_sheet.load());
  }

  void _rebuild() => setState(() {});

  @override
  void dispose() {
    _comment.dispose();
    _title.dispose();
    _query.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return ListenableBuilder(
      listenable: _sheet,
      builder: (context, _) {
        final selected = _sheet.selected;
        final canSend = _sheet.canSend(
          comment: _comment.text,
          title: _title.text,
        );
        final reason = _reasonText(l, _sheet.refusal);
        final targets = _sheet.targets;
        return Scaffold(
          key: const ValueKey('share.sheet'),
          backgroundColor: colors.surfaceCanvas,
          body: SafeArea(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: DsSpace.s8,
                    vertical: DsSpace.s8,
                  ),
                  // One line when it fits; with large text or long words the three parts
                  // stack instead of overflowing.
                  child: OverflowBar(
                    alignment: MainAxisAlignment.spaceBetween,
                    overflowAlignment: OverflowBarAlignment.center,
                    overflowSpacing: DsSpace.s4,
                    children: [
                      DsButton(
                        key: const ValueKey('share.cancel'),
                        label: l.shareCancel,
                        style: DsButtonStyle.ghost,
                        onPressed: widget.onCancel,
                      ),
                      Semantics(
                        header: true,
                        child: Text(
                          l.shareSheetTitle,
                          textAlign: TextAlign.center,
                          style: dsTextStyle(DsType.heading)
                              .copyWith(color: colors.textPrimary),
                        ),
                      ),
                      DsButton(
                        key: const ValueKey('share.send'),
                        label: l.shareSend,
                        style: DsButtonStyle.primary,
                        loading: _sheet.sending,
                        onPressed: canSend
                            ? () => unawaited(
                                _sheet.send(
                                  widget.onSend,
                                  comment: _comment.text,
                                  title: _title.text,
                                ),
                              )
                            : null,
                      ),
                    ],
                  ),
                ),
                const Divider(height: 1),
                Expanded(
                  child: ListView(
                    padding: const EdgeInsets.all(DsSpace.s16),
                    // A form, not a list of items: without semantic indexes every child is no
                    // node of its own, so the gaps between the fields are no empty stops for
                    // TalkBack and VoiceOver (brief M5-10).
                    addSemanticIndexes: false,
                    children: [
                      if (_sheet.files.isNotEmpty) _preview(l, colors),
                      if (reason != null) ...[
                        const SizedBox(height: DsSpace.s6),
                        Text(
                          reason,
                          key: const ValueKey('share.reason'),
                          style: dsTextStyle(DsType.bodySm)
                              .copyWith(color: colors.dangerFg),
                        ),
                      ],
                      const SizedBox(height: DsSpace.s12),
                      TextField(
                        key: const ValueKey('share.comment'),
                        controller: _comment,
                        minLines: 1,
                        maxLines: 4,
                        decoration: InputDecoration(labelText: l.shareComment),
                      ),
                      if (selected?.kind == ShareTargetKind.newPost) ...[
                        const SizedBox(height: DsSpace.s12),
                        TextField(
                          key: const ValueKey('share.title'),
                          controller: _title,
                          decoration: InputDecoration(
                            labelText: l.sharePostTitle,
                          ),
                        ),
                      ],
                      const SizedBox(height: DsSpace.s16),
                      Text(
                        l.shareTargets,
                        style: dsTextStyle(DsType.label)
                            .copyWith(color: colors.textSecondary),
                      ),
                      const SizedBox(height: DsSpace.s6),
                      for (final (i, target) in targets.indexed)
                        _targetRow(l, colors, i, target, target == selected),
                      if (targets.isEmpty)
                        Padding(
                          padding: const EdgeInsets.symmetric(
                            vertical: DsSpace.s8,
                          ),
                          child: Text(
                            l.shareNoTarget,
                            key: const ValueKey('share.empty'),
                            style: dsTextStyle(DsType.bodySm)
                                .copyWith(color: colors.textSecondary),
                          ),
                        ),
                      const SizedBox(height: DsSpace.s8),
                      TextField(
                        key: const ValueKey('share.search'),
                        controller: _query,
                        onChanged: (q) => unawaited(_sheet.setQuery(q)),
                        decoration: InputDecoration(
                          hintText: l.shareSearch,
                          prefixIcon: Padding(
                            padding: const EdgeInsets.all(DsSpace.s8),
                            child: DsIcon(
                              DsIcons.actionSearch,
                              size: DsIconSizeRole.sm,
                              decorative: true,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(height: DsSpace.s16),
                      Text(
                        l.shareWriteOnly,
                        key: const ValueKey('share.note'),
                        style: dsTextStyle(DsType.caption)
                            .copyWith(color: colors.textSecondary),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _preview(AppLocalizations l, DsColors colors) {
    final first = _sheet.files.first;
    final locale = Localizations.localeOf(context).languageCode;
    final more = _sheet.files.length - 1;
    final size = fileSizeLabel(l, first.sizeBytes, locale: locale);
    // One node that says what it is: a bare file name is no human-readable label for a screen
    // reader (the audit of Xcode reports it, brief M5-10).
    return Semantics(
      container: true,
      label: [
        l.sharePreviewLabel(dsIsolate(first.name), size),
        if (more > 0) l.shareMoreFiles(more),
      ].join(', '),
      excludeSemantics: true,
      child: Container(
        key: const ValueKey('share.preview'),
        padding: const EdgeInsets.all(DsSpace.s8),
        decoration: BoxDecoration(
          color: colors.surfaceSunken,
          borderRadius: BorderRadius.circular(DsRadius.sm),
        ),
        child: Row(
          children: [
            DsIcon(
              DsIcons.actionAttach,
              size: DsIconSizeRole.md,
              color: colors.textSecondary,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    dsIsolate(first.name),
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.bodyStrong)
                        .copyWith(color: colors.textPrimary),
                  ),
                  Text(
                    size,
                    style: dsTextStyle(DsType.caption)
                        .copyWith(color: colors.textSecondary),
                  ),
                  if (more > 0)
                    Text(
                      l.shareMoreFiles(more),
                      style: dsTextStyle(DsType.caption)
                          .copyWith(color: colors.textSecondary),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _targetRow(
    AppLocalizations l,
    DsColors colors,
    int index,
    ShareTarget target,
    bool chosen,
  ) {
    final path = dsIsolate(target.path);
    return ListTile(
      key: ValueKey('share.target.$index'),
      contentPadding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
      selected: chosen,
      leading: DsIcon(switch (target.kind) {
        ShareTargetKind.thread => DsIcons.thread,
        ShareTargetKind.channel => DsIcons.channelText,
        ShareTargetKind.newPost => DsIcons.channelForum,
      }, decorative: true),
      title: Text(
        target.kind == ShareTargetKind.newPost ? l.shareNewPost(path) : path,
        overflow: TextOverflow.ellipsis,
      ),
      onTap: () => _sheet.select(target),
    );
  }

  String? _reasonText(AppLocalizations l, AttachRefusal? refusal) {
    final locale = Localizations.localeOf(context).languageCode;
    return switch (refusal) {
      FileTooLarge(:final name, :final limitBytes) => l.shareFileTooLarge(
        dsIsolate(name),
        fileSizeLabel(l, limitBytes, locale: locale),
      ),
      TooManyFiles(:final limit) => l.shareTooManyFiles(limit),
      _ => null,
    };
  }
}
