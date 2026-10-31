// SPDX-License-Identifier: Apache-2.0

/// The safe Markdown renderer of messages (SEC-OUT-1, SEC-OUT-2; docs/DECISIONS.md D-M4-07-2).
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:markdown/markdown.dart' as md;

import '../../generated/design_tokens.g.dart';
import '../../theme/ds_theme.dart';
import '../ds_clip_to_height.dart';
import '../ds_common.dart';
import '../ds_focus_ring.dart';
import 'block_copy.dart';
import 'code_block_scope.dart';
import 'code_lines.dart';
import 'link_policy.dart';

export 'block_copy.dart';
export 'code_block_scope.dart';
export 'code_lines.dart';
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
/// code, fenced code (language label, wrap or horizontal scroll, collapse, copy; 01 6.14),
/// block quote (copy), ordered and unordered lists, read-only task boxes, headings (at most
/// `type.heading`), horizontal rule, tables (horizontal scroll) and links is built from the AST:
///
/// * raw HTML (`<script>`, `<img>`, any tag) shows as its literal text;
/// * an image shows as the text "Image: {alt} ({domain})"; only [attachments] may turn a
///   reference into a widget;
/// * a link is active only for `https`, `http`, `mailto` and the internal paths `/t/...` and
///   `/c/...` ([dsClassifyLink]); its target shows in a tooltip on hover and focus, and
///   [onOpenLink] is called only on a tap, Enter or Space. Any other link is plain text.
///
/// Every code block and every quote block copies exactly itself (01 6.12, R-PR-25): on the
/// desktop through a copy button at its top right, on the phone (density `touch`) through a
/// tap on the block, with "Copied" and a haptic. The copy goes to the [DsClipboardScope]'s
/// clipboard ([dsCodeBlockText], [dsQuoteCopy]); a long press is left to the message around.
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
        return DsMarkdownQuoteBlock(
          content: () => dsQuoteCopy(e),
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
    final text = dsCodeBlockText(pre);
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

/// Whether the ambient density is `touch` (the phone, 01 11): the only density whose
/// `density.target-min` is the touch target. Compared by value, because an animated density
/// switch leaves a lerped `DsDensity` in the theme.
bool _touch(BuildContext context) =>
    context.dsDensity.targetMin >= DsDensity.touch.targetMin;

/// Writes [data] to the clipboard of [context] and confirms it with "Copied"; on the phone
/// ([touch]) with a light haptic too (01 6.12 items 1 and 2).
Future<void> _copyBlock(
  BuildContext context,
  DsCopyData data, {
  required bool touch,
}) async {
  final copied = dsL10n(context).messageCopied;
  // The snack bar needs a scaffold; a renderer outside one (a test, a bare dialog) copies
  // without it.
  final messenger = Scaffold.maybeOf(context) == null
      ? null
      : ScaffoldMessenger.maybeOf(context);
  await DsClipboardScope.of(context).write(data);
  if (touch) await HapticFeedback.lightImpact();
  messenger
    ?..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(content: Text(copied)));
}

/// The small text button at the top right of a block on the desktop.
///
/// Kept out of the system selection, so a selection across the block never copies its label.
class _CopyButton extends StatelessWidget {
  const _CopyButton({required this.label, required this.onTap});

  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return SelectionContainer.disabled(
      child: DsTappable(
        label: label,
        excludeChildSemantics: true,
        onTap: onTap,
        child: Center(
          widthFactor: 1,
          heightFactor: 1,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
            child: Text(
              label,
              style: dsTextStyle(DsType.label)
                  .copyWith(color: context.dsAccent.textLink),
            ),
          ),
        ),
      ),
    );
  }
}

/// On the phone: [child] as one tap target that copies the block, with [label] as the
/// screen reader's name of that action. A long press and drags pass to the widgets around
/// and inside (the message menu, the horizontal scroll of code); a deeper target (a link, a
/// nested block) wins its own tap.
class _TapToCopy extends StatelessWidget {
  const _TapToCopy({
    required this.label,
    required this.onTap,
    required this.child,
  });

  final String label;
  final VoidCallback onTap;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      label: label,
      onTap: onTap,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        excludeFromSemantics: true,
        onTap: onTap,
        child: child,
      ),
    );
  }
}

/// A block quote of [DsMarkdown]: [child] behind a spine in `color.border.default`, which
/// copies [content] (the quote without its `>` markers, as plain text and HTML, [dsQuoteCopy]):
/// through the button "Copy quote" at its top right on the desktop, through a tap on the
/// phone (01 6.12).
class DsMarkdownQuoteBlock extends StatelessWidget {
  /// Creates the block.
  const DsMarkdownQuoteBlock({
    super.key,
    required this.content,
    required this.child,
  });

  /// Builds the copy when the person copies; the parse tree is not converted on every build.
  final DsCopyData Function() content;

  /// The rendered blocks of the quote.
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final touch = _touch(context);
    void copy() => _copyBlock(context, content(), touch: touch);
    final quote = Container(
      padding: const EdgeInsets.only(left: DsSpace.s12),
      decoration: BoxDecoration(
        border: Border(
          left: BorderSide(
            color: context.dsColors.borderDefault,
            width: DsBorderWidth.spine,
          ),
        ),
      ),
      child: touch
          ? child
          : Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(child: child),
                _CopyButton(label: l.markdownCopyQuote, onTap: copy),
              ],
            ),
    );
    return touch
        ? _TapToCopy(label: l.markdownCopyQuote, onTap: copy, child: quote)
        : quote;
  }
}

/// A code block of [DsMarkdown]: `type.code` on `color.code.bg` with its language label, the
/// copy of exactly [code] (the button "Copy code" at its top right on the desktop, a tap on the
/// block on the phone; 01 6.12) and the readable layout of 01 6.14 (M4-49):
///
/// * long lines wrap by default, with a hanging indent for indented lines ([DsCodeLines]);
/// * the toggle "Wrap lines" in the header switches to one line per line with a horizontal
///   scroll: a visible scrollbar on the desktop, shift and the mouse wheel, the trackpad, a drag
///   on the phone. The choice goes to [DsCodeBlockScope.onWrapChanged], so every block of the
///   device follows it; without a callback only this block switches;
/// * a block higher than [DsCodeBlockScope.collapseLines] lines shows collapsed with a fade and
///   "Show all", which expands it in place, and "Show less" collapses it again.
///
/// Wrapping, scrolling and collapsing are display only: the copy is [code] as it is.
class DsMarkdownCodeBlock extends StatefulWidget {
  /// Creates the block.
  const DsMarkdownCodeBlock({super.key, required this.code, this.language});

  /// The key of the fade over the bottom of a collapsed block (01 6.14 item 2).
  static const Key fadeKey = ValueKey('ds-code-block-fade');

  /// The raw code, as [dsCodeBlockText] gives it.
  final String code;

  /// The language of the fence, or null.
  final String? language;

  @override
  State<DsMarkdownCodeBlock> createState() => _DsMarkdownCodeBlockState();
}

class _DsMarkdownCodeBlockState extends State<DsMarkdownCodeBlock> {
  final ScrollController _scroll = ScrollController();

  /// This block's choice from its toggle until the scope reports the same value; the block's
  /// own choice for good when nothing stores it.
  bool? _wrapChoice;

  /// Whether the person expanded the long block ("Show all").
  bool _expanded = false;

  /// Whether the content is higher than the limit, from the last layout.
  bool _overflows = false;

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  bool _wrap(DsCodeBlockScope? scope) {
    final stored = scope?.wrap ?? true;
    // The stored choice caught up with this block's toggle: from now on the block follows the
    // device again, also when another block switches it.
    if (scope?.onWrapChanged != null && _wrapChoice == stored) {
      _wrapChoice = null;
    }
    return _wrapChoice ?? stored;
  }

  void _toggleWrap(DsCodeBlockScope? scope, bool wrap) {
    setState(() => _wrapChoice = !wrap);
    scope?.onWrapChanged?.call(!wrap);
  }

  void _reportOverflow(bool overflows) {
    if (overflows == _overflows) return;
    // The report comes during layout; the control below the block follows in the next frame.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && overflows != _overflows) {
        setState(() => _overflows = overflows);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final l = dsL10n(context);
    final touch = _touch(context);
    final scope = DsCodeBlockScope.maybeOf(context);
    final wrap = _wrap(scope);
    final code = widget.code;
    void copy() => _copyBlock(context, DsCopyData(code), touch: touch);
    final style = dsTextStyle(DsType.code).copyWith(color: colors.codeFg);
    final lineHeight = _lineHeight(style, MediaQuery.textScalerOf(context));
    final label = Text(
      widget.language ?? '',
      style: dsTextStyle(DsType.codeSm).copyWith(color: colors.textSecondary),
    );
    final toggle = _WrapToggle(
      label: l.markdownCodeWrap,
      wrap: wrap,
      onTap: () => _toggleWrap(scope, wrap),
    );
    final header = touch
        ? Row(
            children: [
              const SizedBox(width: DsSpace.s8),
              Expanded(child: label),
              toggle,
              const SizedBox(width: DsSpace.s4),
            ],
          )
        // The copy button stays the first stop of the keyboard (M4-48), the toggle the second.
        : FocusTraversalGroup(
            policy: OrderedTraversalPolicy(),
            child: Row(
              children: [
                const SizedBox(width: DsSpace.s8),
                Expanded(child: label),
                FocusTraversalOrder(
                  order: const NumericFocusOrder(2),
                  child: toggle,
                ),
                FocusTraversalOrder(
                  order: const NumericFocusOrder(1),
                  child: _CopyButton(label: l.markdownCopyCode, onTap: copy),
                ),
              ],
            ),
          );

    final collapsed = _overflows && !_expanded;
    Widget text = DsClipToHeight(
      maxHeight:
          lineHeight *
          (scope?.collapseLines ?? DsCodeBlockScope.defaultCollapseLines),
      clip: !_expanded,
      onOverflow: _reportOverflow,
      child: wrap
          ? DsCodeLines(code: code, style: style)
          : Text(code, softWrap: false, style: style),
    );
    if (collapsed) {
      text = Stack(
        children: [
          text,
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            height: lineHeight * 2,
            child: IgnorePointer(
              child: DecoratedBox(
                key: DsMarkdownCodeBlock.fadeKey,
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [colors.codeBg.withValues(alpha: 0), colors.codeBg],
                  ),
                ),
              ),
            ),
          ),
        ],
      );
    }
    final Widget body;
    if (wrap) {
      body = Padding(
        padding: const EdgeInsets.fromLTRB(
          DsSpace.s12,
          0,
          DsSpace.s12,
          DsSpace.s8,
        ),
        child: text,
      );
    } else {
      // The default scroll behavior draws no scrollbar on a horizontal scroll, so the block
      // brings its own: always visible on the desktop, while scrolling on the phone. The room
      // below the text keeps the thumb off the last line.
      body = Scrollbar(
        controller: _scroll,
        thumbVisibility: !touch,
        child: SingleChildScrollView(
          controller: _scroll,
          primary: false,
          scrollDirection: Axis.horizontal,
          padding: EdgeInsets.fromLTRB(
            DsSpace.s12,
            0,
            DsSpace.s12,
            touch ? DsSpace.s8 : DsSpace.s16,
          ),
          child: text,
        ),
      );
    }
    final more = _overflows
        ? SelectionContainer.disabled(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(
                DsSpace.s4,
                0,
                DsSpace.s4,
                DsSpace.s4,
              ),
              child: Align(
                alignment: AlignmentDirectional.centerStart,
                child: DsTappable(
                  label: _expanded
                      ? l.markdownCodeShowLess
                      : l.markdownCodeShowAll,
                  expanded: _expanded,
                  excludeChildSemantics: true,
                  onTap: () => setState(() => _expanded = !_expanded),
                  child: Center(
                    widthFactor: 1,
                    heightFactor: 1,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: DsSpace.s8,
                      ),
                      child: Text(
                        _expanded
                            ? l.markdownCodeShowLess
                            : l.markdownCodeShowAll,
                        style: dsTextStyle(DsType.label)
                            .copyWith(color: context.dsAccent.textLink),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          )
        : null;
    final block = Container(
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
        // The code is its own node for the screen reader, as the scroll view made it before,
        // so the block's label stays "Copy code" on the phone (01 6.12 item 2).
        children: [
          header,
          Semantics(container: true, child: body),
          ?more,
        ],
      ),
    );
    return touch
        ? _TapToCopy(label: l.markdownCopyCode, onTap: copy, child: block)
        : block;
  }
}

/// The height of one line of [style] under [scaler]: the unit of the collapse limit.
double _lineHeight(TextStyle style, TextScaler scaler) {
  final painter = TextPainter(
    text: TextSpan(text: 'X', style: style),
    textDirection: TextDirection.ltr,
    textScaler: scaler,
  )..layout();
  final height = painter.preferredLineHeight;
  painter.dispose();
  return height;
}

/// The toggle "Wrap lines" in the header of a code block: a small text button, highlighted and
/// read as "on" by the screen reader while lines wrap (01 6.14 item 1).
///
/// Kept out of the system selection, so a selection across the block never copies its label.
class _WrapToggle extends StatelessWidget {
  const _WrapToggle({
    required this.label,
    required this.wrap,
    required this.onTap,
  });

  final String label;
  final bool wrap;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return SelectionContainer.disabled(
      child: DsTappable(
        label: label,
        toggled: wrap,
        excludeChildSemantics: true,
        onTap: onTap,
        child: Center(
          widthFactor: 1,
          heightFactor: 1,
          child: Container(
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s6,
              vertical: DsSpace.s2,
            ),
            decoration: BoxDecoration(
              color: wrap ? colors.surfaceSelected : null,
              borderRadius: BorderRadius.circular(DsRadius.sm),
            ),
            child: Text(
              label,
              style: dsTextStyle(DsType.label).copyWith(
                color: wrap ? context.dsAccent.textLink : colors.textSecondary,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
