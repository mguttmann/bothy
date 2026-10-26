// SPDX-License-Identifier: Apache-2.0

/// The safe Markdown renderer of messages (SEC-OUT-1, SEC-OUT-2; docs/DECISIONS.md D-M4-07-2).
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:markdown/markdown.dart' as md;

import '../../generated/design_tokens.g.dart';
import '../../theme/ds_theme.dart';
import '../ds_common.dart';
import '../ds_focus_ring.dart';
import 'link_policy.dart';

export 'link_policy.dart';

/// Resolves an image reference of a message to a widget, or returns null for the text form.
///
/// Only the caller knows which references are server files (the internal reference form of
/// M4-22); it must return null for every other URL, because the widget it returns is drawn
/// as it is. The renderer itself never loads anything.
typedef DsAttachmentResolver = Widget? Function(Uri reference);

/// Renders the Markdown [source] of a message as widgets, without HTML and without loading
/// anything.
///
/// The source is parsed with package `markdown` (GitHub flavored, without the HTML block and
/// inline HTML syntaxes) and the subset paragraph, emphasis, strong, strikethrough, inline
/// code, fenced code (language label, horizontal scroll, copy button), block quote, ordered
/// and unordered lists, read-only task boxes, headings (at most `type.heading`), horizontal
/// rule, tables (horizontal scroll) and links is built from the AST:
///
/// * raw HTML (`<script>`, `<img>`, any tag) shows as its literal text;
/// * an image shows as the text "Image: {alt} ({domain})"; only [attachments] may turn a
///   reference into a widget;
/// * a link is active only for `https`, `http`, `mailto` and the internal paths `/t/...` and
///   `/c/...` ([dsClassifyLink]); its target shows in a tooltip on hover and focus, and
///   [onOpenLink] is called only on a tap, Enter or Space. Any other link is plain text.
///
/// [style] is the body style, by default `type.body` in `color.text.primary`.
///
/// ```dart
/// DsMarkdown(message.content, onOpenLink: (uri) => router.open(uri));
/// ```
class DsMarkdown extends StatelessWidget {
  /// Creates the renderer for [source].
  const DsMarkdown(
    this.source, {
    super.key,
    required this.onOpenLink,
    this.attachments,
    this.style,
    this.mentionHandles = const {},
  });

  /// The Markdown text, unchanged content of the message.
  final String source;

  /// Called with the target of an active link when the person activates it.
  final void Function(Uri) onOpenLink;

  /// Resolves server file references of images, or null.
  final DsAttachmentResolver? attachments;

  /// The body text style, or null for `type.body`.
  final TextStyle? style;

  /// The handles (lower case, without `@`) that the server resolved for this message
  /// (`mention_ids`); a token `@handle` outside code renders as a mention only when its handle
  /// is here (03 content format items 1, 2 and 4). Empty: no token is a mention.
  final Set<String> mentionHandles;

  /// A mention token of 03 content format item 1: `@` after the start or a character that is
  /// no letter, digit or one of `_ . - @ /`, then a handle; trailing `.`, `-` and `_` end it.
  static final RegExp mentionToken = RegExp(
    r'(?<![\p{L}\p{N}_.\-@/])@([a-z0-9](?:[a-z0-9._-]*[a-z0-9])?)',
    caseSensitive: false,
    unicode: true,
  );

  /// The deepest nesting of block quotes and lists that renders as structure; a deeper
  /// element renders as its plain text. The server accepts messages without a nesting limit,
  /// and every level is a widget subtree, so thousands of levels would freeze the interface.
  static const int maxNesting = 8;

  /// Parses [source] into the AST nodes the renderer draws.
  ///
  /// Public for tests and for callers that need to know the structure (for example to count
  /// lines); the result never contains an HTML node, because the HTML syntaxes are left out.
  static List<md.Node> parse(String source) {
    final document = md.Document(
      extensionSet: md.ExtensionSet.none,
      blockSyntaxes: const [
        md.EmptyBlockSyntax(),
        md.FencedCodeBlockSyntax(),
        md.SetextHeaderSyntax(),
        md.HeaderSyntax(),
        md.CodeBlockSyntax(),
        md.BlockquoteSyntax(),
        md.HorizontalRuleSyntax(),
        md.TableSyntax(),
        md.UnorderedListWithCheckboxSyntax(),
        md.OrderedListWithCheckboxSyntax(),
        md.LinkReferenceDefinitionSyntax(),
        md.ParagraphSyntax(),
      ],
      inlineSyntaxes: [md.StrikethroughSyntax(), md.AutolinkExtensionSyntax()],
      encodeHtml: false,
      withDefaultBlockSyntaxes: false,
    );
    return document.parse(source.replaceAll('\r\n', '\n'));
  }

  @override
  Widget build(BuildContext context) {
    final base =
        style ??
        dsTextStyle(DsType.body).copyWith(color: context.dsColors.textPrimary);
    final builder = _Builder(
      context,
      base,
      onOpenLink,
      attachments,
      mentionHandles,
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: builder.blocks(parse(source)),
    );
  }
}

/// Builds the widgets of the AST.
class _Builder {
  _Builder(
    this.context,
    this.base,
    this.onOpenLink,
    this.attachments,
    this.mentionHandles,
  );

  final BuildContext context;
  final TextStyle base;
  final void Function(Uri) onOpenLink;
  final DsAttachmentResolver? attachments;
  final Set<String> mentionHandles;

  /// [text] as spans in [style], with the tokens of [mentionHandles] in the mention style.
  void _text(String text, TextStyle style, List<InlineSpan> out) {
    if (mentionHandles.isEmpty) {
      out.add(TextSpan(text: text, style: style));
      return;
    }
    var at = 0;
    for (final m in DsMarkdown.mentionToken.allMatches(text)) {
      if (!mentionHandles.contains(m[1]!.toLowerCase())) continue;
      if (m.start > at) {
        out.add(TextSpan(text: text.substring(at, m.start), style: style));
      }
      out.add(
        TextSpan(
          text: m[0],
          style: style.copyWith(
            fontWeight: FontWeight.w600,
            color: context.dsAccent.textLink,
          ),
        ),
      );
      at = m.end;
    }
    if (at < text.length) {
      out.add(TextSpan(text: text.substring(at), style: style));
    }
  }

  static const _blockTags = {
    'p',
    'h1',
    'h2',
    'h3',
    'h4',
    'h5',
    'h6',
    'pre',
    'blockquote',
    'ul',
    'ol',
    'hr',
    'table',
  };

  List<Widget> blocks(List<md.Node> nodes) {
    final out = <Widget>[];
    final pendingInline = <md.Node>[];
    void flushInline() {
      if (pendingInline.isEmpty) return;
      out.add(_paragraph(List.of(pendingInline), base));
      pendingInline.clear();
    }

    for (final node in nodes) {
      if (node is md.Element && _blockTags.contains(node.tag)) {
        flushInline();
        if (out.isNotEmpty) out.add(const SizedBox(height: DsSpace.s8));
        out.add(_block(node));
      } else {
        pendingInline.add(node);
      }
    }
    flushInline();
    return out;
  }

  /// The current nesting of block quotes and lists.
  int _depth = 0;

  Widget _block(md.Element e) {
    final nests = e.tag == 'blockquote' || e.tag == 'ul' || e.tag == 'ol';
    if (!nests) return _blockAt(e);
    if (_depth >= DsMarkdown.maxNesting) {
      return Text(e.textContent, style: base);
    }
    _depth++;
    try {
      return _blockAt(e);
    } finally {
      _depth--;
    }
  }

  Widget _blockAt(md.Element e) {
    final colors = context.dsColors;
    switch (e.tag) {
      case 'h1' || 'h2' || 'h3':
        return _paragraph(
          e.children ?? const [],
          dsTextStyle(DsType.heading).copyWith(color: colors.textPrimary),
          header: true,
        );
      case 'h4' || 'h5' || 'h6':
        return _paragraph(
          e.children ?? const [],
          dsTextStyle(DsType.bodyStrong).copyWith(color: colors.textPrimary),
          header: true,
        );
      case 'pre':
        return _code(e);
      case 'blockquote':
        return Container(
          padding: const EdgeInsets.only(left: DsSpace.s12),
          decoration: BoxDecoration(
            border: Border(
              left: BorderSide(
                color: colors.borderDefault,
                width: DsBorderWidth.spine,
              ),
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: blocks(e.children ?? const []),
          ),
        );
      case 'ul' || 'ol':
        return _list(e);
      case 'hr':
        return Divider(
          height: DsSpace.s16,
          thickness: DsBorderWidth.hairline,
          color: colors.borderSubtle,
        );
      case 'table':
        return _table(e);
      default:
        return _paragraph(e.children ?? const [], base);
    }
  }

  Widget _code(md.Element pre) {
    final code = pre.children?.whereType<md.Element>().firstOrNull;
    var text = (code ?? pre).textContent;
    if (text.endsWith('\n')) text = text.substring(0, text.length - 1);
    final cls = code?.attributes['class'];
    final language = cls != null && cls.startsWith('language-')
        ? cls.substring('language-'.length)
        : null;
    return DsMarkdownCodeBlock(code: text, language: language);
  }

  Widget _list(md.Element list) {
    final ordered = list.tag == 'ol';
    final start = int.tryParse(list.attributes['start'] ?? '') ?? 1;
    final items = (list.children ?? const []).whereType<md.Element>().toList();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (var i = 0; i < items.length; i++)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s4),
            child: _listItem(items[i], ordered ? '${start + i}.' : '•'),
          ),
      ],
    );
  }

  Widget _listItem(md.Element item, String marker) {
    final children = List<md.Node>.of(item.children ?? const []);
    Widget? lead;
    // In a loose list (blank lines between items) the box sits inside the first paragraph.
    md.Element? box;
    final first = children.isEmpty ? null : children.first;
    if (first is md.Element && first.tag == 'input') {
      box = children.removeAt(0) as md.Element;
    } else if (first is md.Element &&
        first.tag == 'p' &&
        (first.children?.isNotEmpty ?? false) &&
        first.children!.first is md.Element &&
        (first.children!.first as md.Element).tag == 'input') {
      box = first.children!.first as md.Element;
      children[0] = md.Element(first.tag, first.children!.sublist(1));
    }
    if (box != null) {
      final checked = box.attributes['checked'] == 'true';
      lead = SizedBox.square(
        dimension: DsIconSize.sizeMd,
        child: Checkbox(
          value: checked,
          onChanged: null,
          materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
          visualDensity: VisualDensity.compact,
        ),
      );
    }
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: DsSpace.s24,
          child:
              lead ??
              Text(
                marker,
                style: base.copyWith(color: context.dsColors.textSecondary),
              ),
        ),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: blocks(children),
          ),
        ),
      ],
    );
  }

  Widget _table(md.Element table) {
    final rows = <List<md.Element>>[];
    final headerRows = <int>{};
    for (final section
        in (table.children ?? const []).whereType<md.Element>()) {
      for (final row
          in (section.children ?? const []).whereType<md.Element>()) {
        if (section.tag == 'thead') headerRows.add(rows.length);
        rows.add((row.children ?? const []).whereType<md.Element>().toList());
      }
    }
    final width = rows.fold<int>(0, (w, r) => r.length > w ? r.length : w);
    if (width == 0) return const SizedBox.shrink();
    final colors = context.dsColors;
    final strong = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Table(
        defaultColumnWidth: const IntrinsicColumnWidth(),
        border: TableBorder.all(
          color: colors.borderSubtle,
          width: DsBorderWidth.hairline,
        ),
        children: [
          for (var r = 0; r < rows.length; r++)
            TableRow(
              children: [
                for (var c = 0; c < width; c++)
                  Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: DsSpace.s8,
                      vertical: DsSpace.s4,
                    ),
                    child: c < rows[r].length
                        ? _paragraph(
                            rows[r][c].children ?? const [],
                            headerRows.contains(r) ? strong : base,
                          )
                        : const SizedBox.shrink(),
                  ),
              ],
            ),
        ],
      ),
    );
  }

  Widget _paragraph(
    List<md.Node> nodes,
    TextStyle style, {
    bool header = false,
  }) {
    final spans = <InlineSpan>[];
    for (final node in nodes) {
      _inline(node, style, spans);
    }
    final block = <Widget>[];
    // Images resolved to widgets by the caller break out of the line.
    final line = <InlineSpan>[];
    for (final span in spans) {
      if (span is _BlockSpan) {
        if (line.isNotEmpty) {
          block.add(Text.rich(TextSpan(children: List.of(line))));
          line.clear();
        }
        block.add(span.widget);
      } else {
        line.add(span);
      }
    }
    if (line.isNotEmpty) {
      block.add(
        Semantics(
          header: header,
          child: Text.rich(TextSpan(style: style, children: line)),
        ),
      );
    }
    if (block.length == 1) return block.single;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: block,
    );
  }

  void _inline(md.Node node, TextStyle style, List<InlineSpan> out) {
    if (node is md.Text) {
      _text(node.text, style, out);
      return;
    }
    if (node is! md.Element) {
      out.add(TextSpan(text: node.textContent, style: style));
      return;
    }
    final colors = context.dsColors;
    void children(TextStyle s) {
      for (final child in node.children ?? const <md.Node>[]) {
        _inline(child, s, out);
      }
    }

    switch (node.tag) {
      case 'strong':
        children(style.copyWith(fontWeight: FontWeight.w600));
      case 'em':
        children(style.copyWith(fontStyle: FontStyle.italic));
      case 'del':
        children(style.copyWith(decoration: TextDecoration.lineThrough));
      case 'code':
        out.add(
          TextSpan(
            text: node.textContent,
            style: dsTextStyle(DsType.code)
                .copyWith(color: colors.codeFg, backgroundColor: colors.codeBg),
          ),
        );
      case 'br':
        out.add(TextSpan(text: '\n', style: style));
      case 'a':
        _link(node, style, out);
      case 'img':
        _image(node, style, out);
      default:
        children(style);
    }
  }

  void _link(md.Element node, TextStyle style, List<InlineSpan> out) {
    final href = node.attributes['href'] ?? '';
    final target = dsClassifyLink(href);
    // An image inside a link keeps its text form, so the link never ends up empty.
    final text = _linkText(node);
    if (target == null) {
      out.add(TextSpan(text: text, style: style));
      return;
    }
    out.add(
      WidgetSpan(
        alignment: PlaceholderAlignment.baseline,
        baseline: TextBaseline.alphabetic,
        child: DsMarkdownLink(
          text: text,
          uri: target.uri,
          target: target.display,
          style: style.copyWith(
            color: context.dsAccent.textLink,
            decoration: TextDecoration.underline,
            decorationColor: context.dsAccent.textLink,
          ),
          onOpen: onOpenLink,
        ),
      ),
    );
  }

  String _linkText(md.Node node) {
    if (node is md.Text) return node.text;
    if (node is! md.Element) return '';
    if (node.tag == 'img') return _imageText(node);
    return (node.children ?? const <md.Node>[]).map(_linkText).join();
  }

  String _imageText(md.Element node) {
    final l = dsL10n(context);
    final alt = node.attributes['alt'] ?? node.textContent;
    final uri = Uri.tryParse(node.attributes['src'] ?? '');
    final domain = uri != null && uri.host.isNotEmpty ? uri.host : null;
    return domain == null
        ? l.markdownImageNoDomain(alt)
        : l.markdownImage(alt, domain);
  }

  void _image(md.Element node, TextStyle style, List<InlineSpan> out) {
    final src = node.attributes['src'] ?? '';
    final uri = Uri.tryParse(src);
    final resolved = uri == null ? null : attachments?.call(uri);
    if (resolved != null) {
      out.add(_BlockSpan(resolved));
      return;
    }
    out.add(
      TextSpan(
        text: _imageText(node),
        style: style.copyWith(color: context.dsColors.textSecondary),
      ),
    );
  }
}

/// A placeholder span that carries a resolved attachment widget out of the line.
class _BlockSpan extends WidgetSpan {
  const _BlockSpan(this.widget) : super(child: widget);

  final Widget widget;
}

/// An active link of [DsMarkdown]: underlined text with its target in a tooltip.
///
/// The tooltip shows [target] on hover and on focus; [onOpen] runs only on a tap, Enter or
/// Space, with [uri].
class DsMarkdownLink extends StatefulWidget {
  /// Creates the link.
  const DsMarkdownLink({
    super.key,
    required this.text,
    required this.uri,
    required this.target,
    required this.style,
    required this.onOpen,
  });

  /// The link text as the message wrote it.
  final String text;

  /// The target handed to [onOpen].
  final Uri uri;

  /// What the tooltip shows: the domain, the address of `mailto` or the internal path.
  final String target;

  /// The text style.
  final TextStyle style;

  /// Called on activation.
  final void Function(Uri) onOpen;

  @override
  State<DsMarkdownLink> createState() => _DsMarkdownLinkState();
}

class _DsMarkdownLinkState extends State<DsMarkdownLink> {
  final _tooltip = GlobalKey<TooltipState>();
  bool _focused = false;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      key: _tooltip,
      message: widget.target,
      excludeFromSemantics: true,
      child: Semantics(
        // Its own node, so the link does not absorb the text of the paragraph around it.
        container: true,
        link: true,
        label: widget.text,
        hint: widget.target,
        // A screen reader activates the link deliberately with its own gesture.
        onTap: () => widget.onOpen(widget.uri),
        excludeSemantics: true,
        // The link draws its own ring; DsFocusRing draws none around it.
        child: DsDrawsFocusRing(
          child: FocusableActionDetector(
            mouseCursor: SystemMouseCursors.click,
            onShowFocusHighlight: (focused) {
              setState(() => _focused = focused);
              if (focused) {
                _tooltip.currentState?.ensureTooltipVisible();
              } else {
                Tooltip.dismissAllToolTips();
              }
            },
            actions: {
              ActivateIntent: CallbackAction<ActivateIntent>(
                onInvoke: (_) {
                  widget.onOpen(widget.uri);
                  return null;
                },
              ),
            },
            child: GestureDetector(
              onTap: () => widget.onOpen(widget.uri),
              child: DecoratedBox(
                decoration: BoxDecoration(
                  border: _focused
                      ? Border.all(
                          color: context.dsAccent.focusRing,
                          width: DsBorderWidth.focus,
                        )
                      : null,
                  borderRadius: BorderRadius.circular(DsRadius.xs),
                ),
                child: Text(widget.text, style: widget.style),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// A code block of [DsMarkdown]: `type.code` on `color.code.bg`, horizontal scroll, language
/// label and a copy button that puts exactly [code] into the clipboard.
class DsMarkdownCodeBlock extends StatelessWidget {
  /// Creates the block.
  const DsMarkdownCodeBlock({super.key, required this.code, this.language});

  /// The code text without the fence and the last line break.
  final String code;

  /// The language of the fence, or null.
  final String? language;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final l = dsL10n(context);
    return Container(
      decoration: BoxDecoration(
        color: colors.codeBg,
        borderRadius: BorderRadius.circular(DsRadius.md),
        border: Border.all(
          color: colors.borderSubtle,
          width: DsBorderWidth.hairline,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              const SizedBox(width: DsSpace.s8),
              Expanded(
                child: Text(
                  language ?? '',
                  style: dsTextStyle(DsType.codeSm)
                      .copyWith(color: colors.textSecondary),
                ),
              ),
              DsTappable(
                label: l.markdownCopyCode,
                excludeChildSemantics: true,
                onTap: () => Clipboard.setData(ClipboardData(text: code)),
                child: Center(
                  widthFactor: 1,
                  heightFactor: 1,
                  child: Padding(
                    padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
                    child: Text(
                      l.markdownCopyCode,
                      style: dsTextStyle(DsType.label)
                          .copyWith(color: context.dsAccent.textLink),
                    ),
                  ),
                ),
              ),
            ],
          ),
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.fromLTRB(
              DsSpace.s12,
              0,
              DsSpace.s12,
              DsSpace.s8,
            ),
            child: Text(
              code,
              softWrap: false,
              style: dsTextStyle(DsType.code).copyWith(color: colors.codeFg),
            ),
          ),
        ],
      ),
    );
  }
}
