// SPDX-License-Identifier: Apache-2.0

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

/// Columns of the grid; each cell shows one semantic icon in its mapped weight and in fill.
const _columns = 6;
const _cellWidth = 184.0;

/// All semantic icons with their token names as captions (test strings, not interface texts).
class IconGrid extends StatelessWidget {
  const IconGrid({super.key});

  @override
  Widget build(BuildContext context) {
    final caption = Theme.of(context).textTheme.labelSmall;
    return SizedBox(
      width: _columns * _cellWidth,
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s8),
        child: Wrap(
          children: [
            for (final MapEntry(key: name, value: glyph)
                in dsSemanticIcons.entries)
              SizedBox(
                width: _cellWidth - DsSpace.s4,
                child: Padding(
                  padding: const EdgeInsets.all(DsSpace.s4),
                  child: Row(
                    children: [
                      DsIcon(glyph, decorative: true),
                      const SizedBox(width: DsSpace.s4),
                      DsIcon(glyph, decorative: true, filled: true),
                      const SizedBox(width: DsSpace.s8),
                      Expanded(
                        child: Text(
                          name,
                          style: caption,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

void main() {
  goldenTest(
    'all semantic icons in their mapped weight and in fill',
    fileName: 'icons',
    pumpBeforeTest: dsPumpIcons,
    builder: () => dsGoldenMatrix(name: 'icons', builder: IconGrid.new),
  );
}
