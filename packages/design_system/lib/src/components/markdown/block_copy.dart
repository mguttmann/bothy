// SPDX-License-Identifier: Apache-2.0

/// What a code block or a quote block of a message puts on the clipboard (01 6.12, R-PR-25,
/// brief M4-48), and the clipboard port the renderer writes through.
library;

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:markdown/markdown.dart' as md;

import 'link_policy.dart';

/// The content of one copy: the plain [text] every clipboard takes, and an optional [html]
/// form for clipboards that hold rich text (a quote block keeps its formatting in a rich
/// editor; a code block has none).
@immutable
final class DsCopyData {
  /// Creates the content.
  const DsCopyData(this.text, {this.html});

  /// The plain text form.
  final String text;

  /// The HTML fragment, or null for plain text only.
  final String? html;

  @override
  bool operator ==(Object other) =>
      other is DsCopyData && other.text == text && other.html == html;

  @override
  int get hashCode => Object.hash(text, html);

  @override
  String toString() =>
      'DsCopyData(${text.length} chars, html: ${html != null})';
}

/// The clipboard the block copies of `DsMarkdown` write to.
///
/// The app provides one that also writes [DsCopyData.html] where the platform clipboard holds
/// rich text; without a [DsClipboardScope] the renderer uses [DsPlainClipboard].
abstract interface class DsClipboard {
  /// Puts [data] on the clipboard; completes when it is there.
  Future<void> write(DsCopyData data);
}

/// Writes only [DsCopyData.text] through Flutter's [Clipboard].
final class DsPlainClipboard implements DsClipboard {
  /// Creates the plain clipboard.
  const DsPlainClipboard();

  @override
  Future<void> write(DsCopyData data) =>
      Clipboard.setData(ClipboardData(text: data.text));
}

/// Hands [clipboard] to the block copies of every `DsMarkdown` below it.
///
/// ```dart
/// DsClipboardScope(clipboard: PlatformRichClipboard(), child: app);
/// ```
class DsClipboardScope extends InheritedWidget {
  /// Creates the scope.
  const DsClipboardScope({
    super.key,
    required this.clipboard,
    required super.child,
  });

  /// The clipboard below this scope.
  final DsClipboard clipboard;

  /// The clipboard of [context]: the nearest scope's, else [DsPlainClipboard].
  ///
  /// Read at the moment of a copy, outside a build, so it registers no dependency.
  static DsClipboard of(BuildContext context) =>
      context.getInheritedWidgetOfExactType<DsClipboardScope>()?.clipboard ??
      const DsPlainClipboard();

  @override
  bool updateShouldNotify(DsClipboardScope oldWidget) =>
      !identical(oldWidget.clipboard, clipboard);
}

/// The raw content of the code block [pre] (a `pre` element of `DsMarkdown.parse`), byte for
/// byte as it stands between the fences (01 6.12 item 3).
///
/// Indentation, tabs, trailing spaces, blank lines and every character are kept; the fence
/// lines and the language tag are not part of it. Only the line break that ends the last
/// content line is dropped, so a one-line block copies without a line break and a block that
/// ends in blank lines keeps them. Line breaks are `\n` (the renderer reads CR LF as LF).
///
/// Example: the block "```sh\n\techo ü\n\n```" copies "\techo ü\n".
String dsCodeBlockText(md.Element pre) {
  final code = pre.children?.whereType<md.Element>().firstOrNull;
  final text = (code ?? pre).textContent;
  return text.endsWith('\n') ? text.substring(0, text.length - 1) : text;
}

/// The copy of the quote block [quote] (a `blockquote` element of `DsMarkdown.parse`) without
/// its `>` markers (01 6.12 item 4).
///
/// The plain text keeps the line breaks of the source; inline Markdown becomes its text
/// (emphasis, inline code and links as their text, an image as its alt text); blocks are
/// separated by an empty line; a list item starts with "- " or its number, a task with
/// "[x] " or "[ ] "; a code block keeps its raw content ([dsCodeBlockText]); a table row is
/// its cells separated by a tab. A nested quote is part of its parent's copy as text.
///
/// The HTML form carries the same content with its formatting: emphasis, inline code, code
/// blocks, lists, tables, headings and the `https`, `http` and `mailto` links the renderer
/// activates ([dsClassifyLink]); any other link, an internal path and every image are text
/// only, so a paste loads nothing. Every text is escaped.
DsCopyData dsQuoteCopy(md.Element quote) {
  final children = quote.children ?? const <md.Node>[];
  return DsCopyData(
    _PlainWriter().blocks(children),
    html: _HtmlWriter().blocks(children),
  );
}

const _blockTags = {
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

bool _isBlock(md.Node node) =>
    node is md.Element && _blockTags.contains(node.tag);

/// Groups [nodes] into blocks: block elements alone, runs of inline nodes together.
List<List<md.Node>> _group(List<md.Node> nodes) {
  final out = <List<md.Node>>[];
  var inline = <md.Node>[];
  for (final node in nodes) {
    if (_isBlock(node)) {
      if (inline.isNotEmpty) out.add(inline);
      inline = <md.Node>[];
      out.add([node]);
    } else {
      inline.add(node);
    }
  }
  if (inline.isNotEmpty) out.add(inline);
  return out;
}

class _PlainWriter {
  String blocks(List<md.Node> nodes) => [
    for (final group in _group(nodes))
      if (group.length == 1 && _isBlock(group.single))
        _block(group.single as md.Element)
      else
        _inlines(group),
  ].where((b) => b.isNotEmpty).join('\n\n');

  String _block(md.Element e) => switch (e.tag) {
    'pre' => dsCodeBlockText(e),
    'blockquote' => blocks(e.children ?? const []),
    'ul' || 'ol' => _list(e),
    'hr' => '',
    'table' => _table(e),
    _ => _inlines(e.children ?? const []),
  };

  String _list(md.Element list) {
    final start = int.tryParse(list.attributes['start'] ?? '') ?? 1;
    final items = (list.children ?? const []).whereType<md.Element>().toList();
    return [
      for (var i = 0; i < items.length; i++)
        _item(items[i], list.tag == 'ol' ? '${start + i}. ' : '- '),
    ].join('\n');
  }

  String _item(md.Element item, String marker) {
    final (box, rest) = _taskBox(item);
    final prefix = box == null ? marker : '$marker${box ? '[x]' : '[ ]'} ';
    final body = _group(rest)
        .map(
          (g) => g.length == 1 && _isBlock(g.single)
              ? _block(g.single as md.Element)
              : _inlines(g),
        )
        .where((b) => b.isNotEmpty)
        .join('\n');
    // Continuation lines of an item line up under its text.
    final indent = ' ' * prefix.length;
    return prefix + body.replaceAll('\n', '\n$indent');
  }

  String _table(md.Element table) {
    final rows = <String>[];
    for (final section
        in (table.children ?? const []).whereType<md.Element>()) {
      for (final row
          in (section.children ?? const []).whereType<md.Element>()) {
        rows.add(
          (row.children ?? const [])
              .whereType<md.Element>()
              .map((c) => _inlines(c.children ?? const []))
              .join('\t'),
        );
      }
    }
    return rows.join('\n');
  }

  String _inlines(List<md.Node> nodes) => nodes.map(_inline).join();

  String _inline(md.Node node) {
    if (node is md.Text) return node.text;
    if (node is! md.Element) return node.textContent;
    return switch (node.tag) {
      'br' => '\n',
      'img' => node.attributes['alt'] ?? '',
      'input' => '',
      _ => _inlines(node.children ?? const []),
    };
  }
}

/// The task box of a list item: whether it is checked (null without one) and the item's
/// remaining children; in a loose list the box sits in the first paragraph.
(bool?, List<md.Node>) _taskBox(md.Element item) {
  final children = List<md.Node>.of(item.children ?? const []);
  final first = children.isEmpty ? null : children.first;
  if (first is md.Element && first.tag == 'input') {
    children.removeAt(0);
    return (first.attributes['checked'] == 'true', children);
  }
  if (first is md.Element &&
      first.tag == 'p' &&
      (first.children?.isNotEmpty ?? false) &&
      first.children!.first is md.Element &&
      (first.children!.first as md.Element).tag == 'input') {
    final box = first.children!.first as md.Element;
    children[0] = md.Element(first.tag, first.children!.sublist(1));
    return (box.attributes['checked'] == 'true', children);
  }
  return (null, children);
}

String _escape(String text) => text
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');

class _HtmlWriter {
  String blocks(List<md.Node> nodes) => [
    for (final group in _group(nodes))
      if (group.length == 1 && _isBlock(group.single))
        _block(group.single as md.Element)
      else
        '<p>${_inlines(group)}</p>',
  ].join();

  String _block(md.Element e) {
    final children = e.children ?? const <md.Node>[];
    return switch (e.tag) {
      'pre' => '<pre><code>${_escape(dsCodeBlockText(e))}</code></pre>',
      'blockquote' => '<blockquote>${blocks(children)}</blockquote>',
      'ul' || 'ol' => _list(e),
      'hr' => '<hr>',
      'table' => _table(e),
      'h1' ||
      'h2' ||
      'h3' ||
      'h4' ||
      'h5' ||
      'h6' => '<${e.tag}>${_inlines(children)}</${e.tag}>',
      _ => '<p>${_inlines(children)}</p>',
    };
  }

  String _list(md.Element list) {
    final start = int.tryParse(list.attributes['start'] ?? '') ?? 1;
    final open = list.tag == 'ol' && start != 1
        ? '<ol start="$start">'
        : '<${list.tag}>';
    final items = (list.children ?? const []).whereType<md.Element>();
    return '$open${items.map(_item).join()}</${list.tag}>';
  }

  String _item(md.Element item) {
    final (box, rest) = _taskBox(item);
    final mark = box == null ? '' : (box ? '[x] ' : '[ ] ');
    final body = _group(rest)
        .map(
          (g) => g.length == 1 && _isBlock(g.single)
              ? _block(g.single as md.Element)
              : _inlines(g),
        )
        .join();
    return '<li>$mark$body</li>';
  }

  String _table(md.Element table) {
    final out = StringBuffer('<table>');
    for (final section
        in (table.children ?? const []).whereType<md.Element>()) {
      for (final row
          in (section.children ?? const []).whereType<md.Element>()) {
        out.write('<tr>');
        for (final cell in (row.children ?? const []).whereType<md.Element>()) {
          final tag = section.tag == 'thead' ? 'th' : 'td';
          out.write('<$tag>${_inlines(cell.children ?? const [])}</$tag>');
        }
        out.write('</tr>');
      }
    }
    out.write('</table>');
    return out.toString();
  }

  String _inlines(List<md.Node> nodes) => nodes.map(_inline).join();

  String _inline(md.Node node) {
    if (node is md.Text) {
      // A soft line break of the source stays a line break in a rich editor.
      return _escape(node.text).replaceAll('\n', '<br>');
    }
    if (node is! md.Element) return _escape(node.textContent);
    final inner = _inlines(node.children ?? const []);
    return switch (node.tag) {
      'strong' => '<strong>$inner</strong>',
      'em' => '<em>$inner</em>',
      'del' => '<del>$inner</del>',
      'code' => '<code>${_escape(node.textContent)}</code>',
      'br' => '<br>',
      'a' => _link(node, inner),
      'img' => _escape(node.attributes['alt'] ?? ''),
      'input' => '',
      _ => inner,
    };
  }

  String _link(md.Element node, String inner) {
    final target = dsClassifyLink(node.attributes['href'] ?? '');
    // An internal path means nothing outside the app: it pastes as its text.
    if (target == null || target.uri.scheme.isEmpty) return inner;
    return '<a href="${_escape(target.uri.toString())}">$inner</a>';
  }
}
