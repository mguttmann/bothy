// SPDX-License-Identifier: Apache-2.0

/// The excerpt of a search hit (docs/spec/14-search.md 4.6): the server's `segments`, or the
/// local snippet in the same form, as one `Text.rich` whose matches are marked. Every segment is
/// plain text: no HTML and no Markdown is interpreted, so `<b>x</b>` in a message shows as
/// those characters (brief M4-23 fixed requirement 12).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import 'search_port.dart';

/// The segments as one text with the matches in bold on the accent's subtle background.
///
/// Example: `SegmentsText(segments: hit.segments, maxLines: 2)`.
class SegmentsText extends StatelessWidget {
  /// Creates the text of [segments]; [maxLines] limits the excerpt, null shows all of it.
  const SegmentsText({super.key, required this.segments, this.maxLines});

  /// The parts of the excerpt in their order.
  final List<SearchSegment> segments;

  /// The most lines shown, or null.
  final int? maxLines;

  @override
  Widget build(BuildContext context) {
    final c = context.dsColors;
    final base = dsTextStyle(DsType.body).copyWith(color: c.textPrimary);
    // Through dsTextStyle again, so the weight also drives the `wght` axis of the font.
    final match = dsTextStyle(DsType.body.copyWith(fontWeight: FontWeight.w700))
        .copyWith(
          color: c.textPrimary,
          backgroundColor: context.dsAccent.accentSubtle,
        );
    return Text.rich(
      TextSpan(
        children: [
          for (final s in segments)
            TextSpan(text: s.text, style: s.match ? match : base),
        ],
      ),
      maxLines: maxLines,
      overflow: maxLines == null ? null : TextOverflow.ellipsis,
    );
  }
}
