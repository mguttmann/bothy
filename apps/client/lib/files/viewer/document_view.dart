// SPDX-License-Identifier: Apache-2.0

/// The document viewer of text attachments (ticket FEAT-markdown-viewer): a Markdown file
/// rendered with the renderer of messages ([DsMarkdown], block by block and lazily), JSON
/// pretty-printed, a small CSV as a table, and every text file as raw monospaced text; with a
/// toolbar (formatted or raw, search, copy all, Save as, open in another app), a table of
/// contents of the headings, the mouse selection of the history and the keys of a reader.
library;

import 'dart:async';
import 'dart:math' as math;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart' show ScrollCacheExtent;
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../attachment_mapping.dart';
import 'document_content.dart';

/// The lines of raw text one row of the lazy list holds.
const int documentLinesPerRow = 100;

/// The width from which the table of contents is a side panel; below it is a sheet.
const double documentWidePanelMinWidth = 900;

/// The width of the side panel of the table of contents.
const double documentTocWidth = 260;

/// The viewer of a loaded document [content] named [name].
///
/// The actions that leave the viewer or touch files are callbacks, so the view stays free of
/// downloads: [onSave] and [onOpenExternally] return the notice to show (or null), [onOpenLink]
/// opens an active link of the document, [resolveLink] activates a relative link (another
/// attachment of the message), and [images] resolves its images.
class DocumentView extends StatefulWidget {
  /// Creates the view.
  const DocumentView({
    super.key,
    required this.name,
    required this.content,
    required this.onClose,
    required this.onOpenLink,
    this.onSave,
    this.onOpenExternally,
    this.resolveLink,
    this.images,
    this.desktop = true,
  });

  /// The file name, the title of the viewer.
  final String name;

  /// The loaded text.
  final DocumentContent content;

  /// Closes the viewer.
  final VoidCallback onClose;

  /// Opens an activated link that is no anchor of the document.
  final void Function(Uri) onOpenLink;

  /// Saves the file; returns the notice.
  final Future<String?> Function()? onSave;

  /// Hands the file to the system; returns the notice.
  final Future<String?> Function()? onOpenExternally;

  /// Activates a relative link (see [DsMarkdown.resolveLink]); anchors `#...` of the
  /// document's headings are active without it.
  final DsLinkTarget? Function(String href)? resolveLink;

  /// Resolves an image of the document to a widget (see [DsMarkdown.attachments]).
  final DsAttachmentResolver? images;

  /// Whether the mouse selection of the desktop (drag beyond the edge scrolls) is used; the
  /// phone uses the system selection.
  final bool desktop;

  @override
  State<DocumentView> createState() => _DocumentViewState();
}

class _DocumentViewState extends State<DocumentView> {
  final ScrollController _scroll = ScrollController();
  final TextEditingController _query = TextEditingController();
  final FocusNode _queryFocus = FocusNode(debugLabel: 'document.search');
  final FocusNode _bodyFocus = FocusNode(debugLabel: 'document.body');

  /// The contexts of the rows built now, for jumps to a row.
  final Map<int, BuildContext> _rows = {};

  late bool _rendered = widget.content.renderable && !widget.content.large;
  bool _searchOpen = false;
  int _active = -1;
  List<int> _counts = const [];
  List<int> _firsts = const [];
  int _total = 0;
  bool? _tocOpen;
  String? _notice;

  DsMarkdownDocument? _doc;
  List<String>? _rawRows;
  List<String>? _prettyRows;

  @override
  void initState() {
    super.initState();
    _query.addListener(_onQuery);
  }

  @override
  void dispose() {
    _query
      ..removeListener(_onQuery)
      ..dispose();
    _scroll.dispose();
    _queryFocus.dispose();
    _bodyFocus.dispose();
    super.dispose();
  }

  DocumentContent get _content => widget.content;

  /// The Markdown of the rendered view: the file, or the table of a CSV.
  DsMarkdownDocument? get _markdown {
    if (!_rendered) return null;
    return switch (_content.kind) {
      DocumentKind.markdown => _doc ??= DsMarkdownDocument.parse(_content.text),
      DocumentKind.csv when _content.csvTable != null =>
        _doc ??= DsMarkdownDocument.parse(_content.csvTable!),
      _ => null,
    };
  }

  /// The rows of the text view: the raw text, or the pretty JSON.
  List<String> get _lines {
    if (_rendered && _content.pretty != null) {
      return _prettyRows ??= _chunk(_content.pretty!);
    }
    return _rawRows ??= _chunk(_content.text);
  }

  static List<String> _chunk(String text) {
    final lines = text.split('\n');
    return [
      for (var i = 0; i < lines.length; i += documentLinesPerRow)
        lines
            .sublist(i, math.min(i + documentLinesPerRow, lines.length))
            .join('\n'),
    ];
  }

  int get _itemCount => _markdown?.length ?? _lines.length;

  /// The language of the syntax colors of the text view.
  String? get _language => switch (_content.kind) {
    DocumentKind.json => 'json',
    DocumentKind.yaml => 'yaml',
    _ => null,
  };

  /// The query of the last count; the listener also hears moves of the cursor.
  String _counted = '';

  void _onQuery() {
    if (_query.text == _counted) return;
    _counted = _query.text;
    _recount();
    setState(() => _active = _total > 0 ? 0 : -1);
    if (_active >= 0) _revealActive();
  }

  void _recount() {
    final q = _query.text;
    final doc = _markdown;
    final counts = <int>[];
    if (q.isNotEmpty) {
      for (var i = 0; i < _itemCount; i++) {
        counts.add(
          doc == null
              ? dsMatchRanges(_lines[i], q).length
              : DsMarkdown.countMatches(context, doc, i, q),
        );
      }
    }
    final firsts = <int>[];
    var total = 0;
    for (final c in counts) {
      firsts.add(total);
      total += c;
    }
    _counts = counts;
    _firsts = firsts;
    _total = total;
  }

  void _setRendered(bool rendered) {
    setState(() {
      _rendered = rendered;
      _rows.clear();
      if (_query.text.isNotEmpty) {
        _recount();
        _active = _total > 0 ? 0 : -1;
      }
    });
    if (_scroll.hasClients) _scroll.jumpTo(0);
  }

  void _openSearch() {
    setState(() => _searchOpen = true);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _queryFocus.requestFocus();
      _query.selection = TextSelection(
        baseOffset: 0,
        extentOffset: _query.text.length,
      );
    });
  }

  void _closeSearch() {
    _query.clear();
    setState(() => _searchOpen = false);
    _bodyFocus.requestFocus();
  }

  void _step(int delta) {
    if (_total == 0) return;
    setState(() => _active = (_active + delta) % _total);
    _revealActive();
  }

  void _revealActive() {
    for (var i = 0; i < _counts.length; i++) {
      if (_active >= _firsts[i] && _active < _firsts[i] + _counts[i]) {
        _reveal(i);
        return;
      }
    }
  }

  /// Scrolls row [index] into view: directly when it is built, else by an estimate of its
  /// offset and again once the rows around it are built.
  void _reveal(int index, [int attempt = 0]) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scroll.hasClients) return;
      final row = _rows[index];
      if (row != null && row.mounted) {
        unawaited(
          Scrollable.ensureVisible(
            row,
            alignment: 0.05,
            alignmentPolicy: ScrollPositionAlignmentPolicy.explicit,
          ),
        );
        return;
      }
      if (attempt >= 8) return;
      final position = _scroll.position;
      final perRow =
          (position.maxScrollExtent + position.viewportDimension) /
          math.max(1, _itemCount);
      _scroll.jumpTo(
        (perRow * index).clamp(0.0, position.maxScrollExtent).toDouble(),
      );
      _reveal(index, attempt + 1);
    });
  }

  /// Scrolls to the heading with [anchor]; false when the document has none.
  bool _toAnchor(String anchor) {
    final doc = _markdown;
    if (doc == null) return false;
    for (final h in doc.headings) {
      if (h.anchor == anchor) {
        _reveal(h.block);
        return true;
      }
    }
    return false;
  }

  void _openLink(Uri uri) {
    if (uri.scheme.isEmpty && uri.path.isEmpty && uri.hasFragment) {
      _toAnchor(Uri.decodeComponent(uri.fragment));
      return;
    }
    widget.onOpenLink(uri);
  }

  DsLinkTarget? _resolveLink(String href) {
    final trimmed = href.trim();
    if (trimmed.startsWith('#') && trimmed.length > 1) {
      final uri = Uri.tryParse(trimmed);
      if (uri != null) return DsLinkTarget(uri, trimmed);
    }
    return widget.resolveLink?.call(href);
  }

  Future<void> _copyAll() async {
    final l = AppLocalizations.of(context);
    final text = _rendered && _content.pretty != null
        ? _content.pretty!
        : _content.text;
    await DsClipboardScope.of(context).write(DsCopyData(text));
    if (mounted) setState(() => _notice = l.messageCopied);
  }

  Future<void> _run(Future<String?> Function()? action) async {
    if (action == null) return;
    final notice = await action();
    if (mounted) setState(() => _notice = notice);
  }

  void _scrollBy(double delta) {
    if (!_scroll.hasClients) return;
    final p = _scroll.position;
    _scroll.jumpTo(
      (p.pixels + delta).clamp(p.minScrollExtent, p.maxScrollExtent).toDouble(),
    );
  }

  KeyEventResult _onBodyKey(FocusNode node, KeyEvent event) {
    if (event is KeyUpEvent || !_scroll.hasClients) {
      return KeyEventResult.ignored;
    }
    final page = _scroll.position.viewportDimension * 0.9;
    const line = 48.0;
    final shift = HardwareKeyboard.instance.isShiftPressed;
    switch (event.logicalKey) {
      case LogicalKeyboardKey.arrowDown:
        _scrollBy(line);
      case LogicalKeyboardKey.arrowUp:
        _scrollBy(-line);
      case LogicalKeyboardKey.space:
        _scrollBy(shift ? -page : page);
      case LogicalKeyboardKey.pageDown:
        _scrollBy(page);
      case LogicalKeyboardKey.pageUp:
        _scrollBy(-page);
      case LogicalKeyboardKey.home:
        _scroll.jumpTo(_scroll.position.minScrollExtent);
      case LogicalKeyboardKey.end:
        _scroll.jumpTo(_scroll.position.maxScrollExtent);
      default:
        return KeyEventResult.ignored;
    }
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final width = MediaQuery.sizeOf(context).width;
    final wide = width >= documentWidePanelMinWidth;
    final headings = _markdown?.headings ?? const <DsMarkdownHeading>[];
    final hasToc = headings.length >= 2;
    final tocOpen = hasToc && wide && (_tocOpen ?? headings.length >= 3);

    final body = Focus(
      focusNode: _bodyFocus,
      autofocus: true,
      onKeyEvent: _onBodyKey,
      child: _list(context),
    );

    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.keyF, meta: true): _openSearch,
        const SingleActivator(LogicalKeyboardKey.keyF, control: true):
            _openSearch,
        const SingleActivator(LogicalKeyboardKey.escape): () =>
            _searchOpen ? _closeSearch() : widget.onClose(),
      },
      child: Semantics(
        key: const ValueKey('document.viewer'),
        scopesRoute: true,
        namesRoute: true,
        explicitChildNodes: true,
        label: l.documentViewerTitle(dsIsolate(widget.name)),
        // Material: the check boxes of task lists and the search field need one.
        child: Material(
          color: colors.surfaceCanvas,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _header(context, hasToc: hasToc, wide: wide, tocOpen: tocOpen),
              if (_searchOpen) _searchBar(context),
              ..._notices(context),
              Divider(
                height: DsBorderWidth.hairline,
                thickness: DsBorderWidth.hairline,
                color: colors.borderSubtle,
              ),
              Expanded(
                child: tocOpen
                    ? Row(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          SizedBox(
                            width: documentTocWidth,
                            child: _toc(context, headings),
                          ),
                          VerticalDivider(
                            width: DsBorderWidth.hairline,
                            thickness: DsBorderWidth.hairline,
                            color: colors.borderSubtle,
                          ),
                          Expanded(child: body),
                        ],
                      )
                    : body,
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _header(
    BuildContext context, {
    required bool hasToc,
    required bool wide,
    required bool tocOpen,
  }) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    Widget button(String key, String label, VoidCallback? onTap) => DsButton(
      key: ValueKey('document.$key'),
      label: label,
      style: DsButtonStyle.ghost,
      onPressed: onTap,
    );
    final headings = _markdown?.headings ?? const <DsMarkdownHeading>[];
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s16,
        DsSpace.s8,
        DsSpace.s8,
        DsSpace.s4,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Expanded(
                child: Semantics(
                  header: true,
                  child: Text(
                    dsIsolate(widget.name),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.heading)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
              ),
              DsTappable(
                key: const ValueKey('document.close'),
                label: l.documentClose,
                excludeChildSemantics: true,
                onTap: widget.onClose,
                child: Center(
                  widthFactor: 1,
                  heightFactor: 1,
                  child: DsIcon(
                    DsIcons.actionDismiss,
                    color: colors.textSecondary,
                    decorative: true,
                  ),
                ),
              ),
            ],
          ),
          // The actions scroll sideways on a narrow screen instead of wrapping under 200 %
          // text into many rows.
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(
              children: [
                if (hasToc)
                  button(
                    'contents',
                    l.documentContents,
                    () => wide
                        ? setState(() => _tocOpen = !tocOpen)
                        : unawaited(_tocSheet(context, headings)),
                  ),
                if (_content.renderable)
                  button(
                    'toggle',
                    _rendered ? l.documentShowRaw : l.documentShowRendered,
                    () => _setRendered(!_rendered),
                  ),
                button('search', l.documentSearch, _openSearch),
                button('copy', l.documentCopyAll, () => unawaited(_copyAll())),
                if (widget.onSave != null)
                  button(
                    'save',
                    l.fileSaveAs,
                    () => unawaited(_run(widget.onSave)),
                  ),
                if (widget.onOpenExternally != null)
                  button(
                    'external',
                    l.documentOpenExternally,
                    () => unawaited(_run(widget.onOpenExternally)),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _searchBar(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final status = _query.text.isEmpty
        ? ''
        : _total == 0
        ? l.documentSearchNone
        : l.documentSearchCount(_active + 1, _total);
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s16,
        0,
        DsSpace.s8,
        DsSpace.s8,
      ),
      child: Row(
        children: [
          Expanded(
            child: CallbackShortcuts(
              bindings: {
                const SingleActivator(
                  LogicalKeyboardKey.enter,
                  shift: true,
                ): () =>
                    _step(-1),
              },
              child: TextField(
                key: const ValueKey('document.search.field'),
                controller: _query,
                focusNode: _queryFocus,
                textInputAction: TextInputAction.search,
                onSubmitted: (_) {
                  _step(1);
                  _queryFocus.requestFocus();
                },
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textPrimary),
                decoration: InputDecoration(
                  isDense: true,
                  hintText: l.documentSearchHint,
                  border: const OutlineInputBorder(),
                ),
              ),
            ),
          ),
          const SizedBox(width: DsSpace.s8),
          Semantics(
            liveRegion: true,
            child: Text(
              status,
              key: const ValueKey('document.search.count'),
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
          DsButton(
            key: const ValueKey('document.search.previous'),
            label: l.documentSearchPrevious,
            style: DsButtonStyle.ghost,
            onPressed: _total == 0 ? null : () => _step(-1),
          ),
          DsButton(
            key: const ValueKey('document.search.next'),
            label: l.documentSearchNext,
            style: DsButtonStyle.ghost,
            onPressed: _total == 0 ? null : () => _step(1),
          ),
          DsTappable(
            key: const ValueKey('document.search.close'),
            label: l.documentSearchClose,
            excludeChildSemantics: true,
            onTap: _closeSearch,
            child: Center(
              widthFactor: 1,
              heightFactor: 1,
              child: DsIcon(
                DsIcons.actionDismiss,
                size: DsIconSizeRole.sm,
                color: colors.textSecondary,
                decorative: true,
              ),
            ),
          ),
        ],
      ),
    );
  }

  List<Widget> _notices(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final locale = Localizations.localeOf(context).languageCode;
    final caption = dsTextStyle(DsType.caption)
        .copyWith(color: colors.textSecondary);
    Widget line(String key, String text, [Widget? action]) => Padding(
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s16,
        0,
        DsSpace.s16,
        DsSpace.s4,
      ),
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        spacing: DsSpace.s8,
        children: [
          Text(text, key: ValueKey('document.$key'), style: caption),
          ?action,
        ],
      ),
    );
    return [
      if (_content.truncated)
        line(
          'truncated',
          l.documentTruncated(
            fileSizeLabel(l, documentMaxLoadBytes, locale: locale),
          ),
        ),
      if (_content.large && !_rendered)
        line(
          'large',
          l.documentLarge(
            fileSizeLabel(l, _content.totalBytes, locale: locale),
          ),
          _content.renderable
              ? DsButton(
                  key: const ValueKey('document.render'),
                  label: l.documentRenderAnyway,
                  style: DsButtonStyle.ghost,
                  onPressed: () => _setRendered(true),
                )
              : null,
        ),
      if (_notice case final notice?) line('notice', notice),
    ];
  }

  Widget _toc(BuildContext context, List<DsMarkdownHeading> headings) {
    final colors = context.dsColors;
    return ListView(
      key: const ValueKey('document.toc'),
      padding: const EdgeInsets.all(DsSpace.s8),
      children: [
        for (var i = 0; i < headings.length; i++)
          DsTappable(
            key: ValueKey('document.toc.$i'),
            onTap: () => _reveal(headings[i].block),
            child: Padding(
              padding: EdgeInsets.fromLTRB(
                DsSpace.s8 + DsSpace.s12 * (headings[i].level - 1).clamp(0, 4),
                DsSpace.s4,
                DsSpace.s8,
                DsSpace.s4,
              ),
              child: Text(
                headings[i].text,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: dsTextStyle(
                  headings[i].level == 1 ? DsType.bodyStrong : DsType.bodySm,
                ).copyWith(color: colors.textPrimary),
              ),
            ),
          ),
      ],
    );
  }

  Future<void> _tocSheet(
    BuildContext context,
    List<DsMarkdownHeading> headings,
  ) async {
    final l = AppLocalizations.of(context);
    final block = await showModalBottomSheet<int>(
      context: context,
      useSafeArea: true,
      isScrollControlled: true,
      builder: (sheet) => FractionallySizedBox(
        heightFactor: 0.7,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.all(DsSpace.s16),
              child: Semantics(
                header: true,
                child: Text(
                  l.documentContents,
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: sheet.dsColors.textPrimary),
                ),
              ),
            ),
            Expanded(
              child: ListView(
                key: const ValueKey('document.toc'),
                children: [
                  for (var i = 0; i < headings.length; i++)
                    DsTappable(
                      key: ValueKey('document.toc.$i'),
                      onTap: () => Navigator.of(sheet).pop(headings[i].block),
                      child: Padding(
                        padding: EdgeInsets.fromLTRB(
                          DsSpace.s16 +
                              DsSpace.s12 * (headings[i].level - 1).clamp(0, 4),
                          DsSpace.s8,
                          DsSpace.s16,
                          DsSpace.s8,
                        ),
                        child: Text(
                          headings[i].text,
                          style: dsTextStyle(DsType.body)
                              .copyWith(color: sheet.dsColors.textPrimary),
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
    if (block != null && mounted) _reveal(block);
  }

  Widget _list(BuildContext context) {
    final doc = _markdown;
    final colors = context.dsColors;
    final query = _query.text;
    DsTextSearch? searchOf(int i) => query.isEmpty || i >= _firsts.length
        ? null
        : DsTextSearch(query, active: _active, first: _firsts[i]);
    final bodyStyle = dsTextStyle(DsType.body).copyWith(
      color: colors.textPrimary,
      fontSize: context.dsDensity.messageFontSize,
    );
    final codeStyle = dsTextStyle(DsType.code)
        .copyWith(color: colors.textPrimary);
    final language = _language;

    Widget row(BuildContext context, int i) {
      final Widget content;
      if (doc != null) {
        content = Padding(
          padding: const EdgeInsets.only(bottom: DsSpace.s12),
          child: DsMarkdown.block(
            doc,
            i,
            onOpenLink: _openLink,
            resolveLink: _resolveLink,
            attachments: widget.images,
            style: bodyStyle,
            search: searchOf(i),
          ),
        );
      } else {
        final text = _lines[i];
        final runs = dsCodeTokens(text, language);
        final (spans, _) = dsMarkMatches(
          text,
          [
            for (final r in runs)
              (r.start, r.end, dsCodeTokenStyle(r.token, codeStyle, colors)),
          ],
          searchOf(i),
          colors,
        );
        content = Text.rich(
          TextSpan(children: spans.isEmpty ? [TextSpan(text: text)] : spans),
          style: codeStyle,
        );
      }
      return _Row(
        index: i,
        rows: _rows,
        child: DsSelectableRow(child: DsSelectableContent(child: content)),
      );
    }

    final list = ListView.builder(
      key: ValueKey(doc != null ? 'document.rendered' : 'document.raw'),
      controller: _scroll,
      // Rows of a selection stay built while it lasts (FIX-drag-select-autoscroll).
      scrollCacheExtent: const ScrollCacheExtent.pixels(2000),
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s16,
        DsSpace.s16,
        DsSpace.s16,
        DsSpace.s48,
      ),
      itemCount: _itemCount,
      itemBuilder: (context, i) => Center(
        child: ConstrainedBox(
          // A readable line length on a wide screen.
          constraints: const BoxConstraints(maxWidth: DsLayout.messageMaxWidth),
          child: row(context, i),
        ),
      ),
    );
    return widget.desktop
        ? DsListSelection(separator: doc != null ? '\n\n' : '\n', child: list)
        : SelectionArea(child: list);
  }
}

/// A row of the lazy list that registers its context while it is built, so a jump can find it.
class _Row extends StatefulWidget {
  const _Row({required this.index, required this.rows, required this.child});

  final int index;
  final Map<int, BuildContext> rows;
  final Widget child;

  @override
  State<_Row> createState() => _RowState();
}

class _RowState extends State<_Row> {
  @override
  void initState() {
    super.initState();
    widget.rows[widget.index] = context;
  }

  @override
  void didUpdateWidget(_Row oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.index != widget.index) {
      if (identical(oldWidget.rows[oldWidget.index], context)) {
        oldWidget.rows.remove(oldWidget.index);
      }
      widget.rows[widget.index] = context;
    }
  }

  @override
  void dispose() {
    if (identical(widget.rows[widget.index], context)) {
      widget.rows.remove(widget.index);
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
