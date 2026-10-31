// SPDX-License-Identifier: Apache-2.0

/// The lines of a wrapped code block with a hanging indent (01 6.14 item 3; brief M4-49).
library;

import 'package:flutter/rendering.dart' show SelectedContent;
import 'package:flutter/widgets.dart';

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
  const DsCodeLines({super.key, required this.code, required this.style});

  /// The raw code, line breaks as LF.
  final String code;

  /// The text style of every line.
  final TextStyle style;

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
    final column = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final line in code.split('\n')) _line(line, maxIndent, selectable),
      ],
    );
    return selectable ? _JoinedSelection(child: column) : column;
  }

  Widget _line(String line, double maxIndent, bool selectable) {
    final indent = _indent.firstMatch(line);
    if (indent == null || indent.end == line.length) {
      return Text(line, style: style);
    }
    final row = Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ConstrainedBox(
          constraints: BoxConstraints(maxWidth: maxIndent),
          child: Text(
            line.substring(0, indent.end),
            style: style,
            softWrap: false,
            maxLines: 1,
          ),
        ),
        Expanded(child: Text(line.substring(indent.end), style: style)),
      ],
    );
    return selectable ? _JoinedSelection(lines: false, child: row) : row;
  }
}

/// A selection container whose content joins its parts with a line break ([lines]) or with
/// nothing (the indentation and the rest of one line).
class _JoinedSelection extends StatefulWidget {
  const _JoinedSelection({this.lines = true, required this.child});

  final bool lines;
  final Widget child;

  @override
  State<_JoinedSelection> createState() => _JoinedSelectionState();
}

class _JoinedSelectionState extends State<_JoinedSelection> {
  late final _JoinDelegate _delegate = _JoinDelegate(widget.lines ? '\n' : '');

  @override
  void dispose() {
    _delegate.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) =>
      SelectionContainer(delegate: _delegate, child: widget.child);
}

class _JoinDelegate extends StaticSelectionContainerDelegate {
  _JoinDelegate(this.separator);

  final String separator;

  @override
  SelectedContent? getSelectedContent() {
    final parts = [
      for (final selectable in selectables)
        if (selectable.getSelectedContent() case final SelectedContent data)
          data.plainText,
    ];
    if (parts.isEmpty) return null;
    return SelectedContent(plainText: parts.join(separator));
  }
}
