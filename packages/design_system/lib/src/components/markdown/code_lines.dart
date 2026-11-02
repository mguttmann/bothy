// SPDX-License-Identifier: Apache-2.0

/// The lines of a wrapped code block with a hanging indent (01 6.14 item 3; brief M4-49).
library;

import 'package:flutter/widgets.dart';

import 'selection_join.dart';

/// Draws [code] line by line, each line wrapping at the width it gets; a line that starts with
/// spaces or tabs keeps that indentation on every wrapped line (hanging indent), so the
/// structure of indented code stays readable while it wraps.
///
/// Flutter's paragraph has no hanging indent, so an indented line is its indentation beside a
/// box with the rest of the line. The indentation takes at most [maxIndentColumns] columns, so
/// deep indentation never squeezes the text into a narrow strip on the phone. Under a system
/// selection the lines join with a line break and an indented line stays one piece, so a
/// selection of the block keeps its line breaks and indentation (01 6.12 item 6). The copy of
/// the block never reads these widgets: it is the raw text of the parse tree (01 6.12 item 3).
///
/// ```dart
/// DsCodeLines(code: 'if (a) {\n  run();\n}', style: dsTextStyle(DsType.code));
/// ```
class DsCodeLines extends StatelessWidget {
  /// Creates the lines of [code].
  const DsCodeLines({
    super.key,
    required this.code,
    required this.style,
    this.spans,
  });

  /// The raw code, line breaks as LF.
  final String code;

  /// The text style of every line.
  final TextStyle style;

  /// The code in styled pieces (syntax colors, search marks), or null for [code] in [style]:
  /// flat spans whose texts join to exactly [code].
  final List<TextSpan>? spans;

  /// The widest hanging indent, in columns of the code font: deeper indentation is kept in
  /// the text but drawn cut at this width, so it never squeezes the rest of the line into a
  /// narrow strip on the phone.
  static const int maxIndentColumns = 16;

  static final RegExp _indent = RegExp(r'^[ \t]+');

  @override
  Widget build(BuildContext context) {
    final selectable = SelectionContainer.maybeOf(context) != null;
    // No LayoutBuilder: a message may sit in a layout that asks for intrinsic sizes.
    final painter = TextPainter(
      text: TextSpan(text: '0' * maxIndentColumns, style: style),
      textDirection: TextDirection.ltr,
      textScaler: MediaQuery.textScalerOf(context),
    )..layout();
    final maxIndent = painter.width;
    painter.dispose();
    final pieces = _Pieces(spans ?? [TextSpan(text: code, style: style)]);
    final lines = <Widget>[];
    var at = 0;
    for (final line in code.split('\n')) {
      lines.add(_line(line, at, pieces, maxIndent, selectable));
      at += line.length + 1;
    }
    final column = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: lines,
    );
    return selectable
        ? DsSelectionJoin(separator: '\n', child: column)
        : column;
  }

  Widget _line(
    String line,
    int offset,
    _Pieces pieces,
    double maxIndent,
    bool selectable,
  ) {
    Text text(int start, int end, {bool softWrap = true, int? maxLines}) {
      final children = pieces.slice(start, end);
      return Text.rich(
        // An empty line keeps the height of a line.
        children.isEmpty
            ? TextSpan(text: '', style: style)
            : TextSpan(children: children),
        style: style,
        softWrap: softWrap,
        maxLines: maxLines,
      );
    }

    if (line.isEmpty && selectable) {
      // An empty paragraph has nothing to select, so a selection over it would lose the
      // empty line; a space that copies as nothing keeps its line break (01 6.15 item 3).
      return DsSelectionJoin.replace(
        replacement: '',
        child: Text(' ', style: style),
      );
    }
    final indent = _indent.firstMatch(line);
    if (indent == null || indent.end == line.length) {
      return text(offset, offset + line.length);
    }
    final row = Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ConstrainedBox(
          constraints: BoxConstraints(maxWidth: maxIndent),
          child: text(
            offset,
            offset + indent.end,
            softWrap: false,
            maxLines: 1,
          ),
        ),
        Expanded(child: text(offset + indent.end, offset + line.length)),
      ],
    );
    return selectable ? DsSelectionJoin(separator: '', child: row) : row;
  }
}

/// Flat text spans with their offsets, cut into ranges.
final class _Pieces {
  _Pieces(this.spans) {
    var at = 0;
    for (final s in spans) {
      starts.add(at);
      at += s.text?.length ?? 0;
    }
  }

  final List<TextSpan> spans;
  final List<int> starts = [];

  /// The spans of the characters from [start] to [end].
  List<TextSpan> slice(int start, int end) {
    final out = <TextSpan>[];
    // The last span that starts at or before [start]; lines are cut in order, so a binary
    // search keeps a long block linear.
    var lo = 0;
    var hi = starts.length - 1;
    while (lo < hi) {
      final mid = (lo + hi + 1) >> 1;
      if (starts[mid] <= start) {
        lo = mid;
      } else {
        hi = mid - 1;
      }
    }
    for (var i = lo; i < spans.length; i++) {
      final text = spans[i].text ?? '';
      final s = starts[i];
      final e = s + text.length;
      if (e <= start) continue;
      if (s >= end) break;
      final from = start > s ? start - s : 0;
      final to = end < e ? end - s : text.length;
      if (to > from) {
        out.add(
          TextSpan(text: text.substring(from, to), style: spans[i].style),
        );
      }
    }
    return out;
  }
}
