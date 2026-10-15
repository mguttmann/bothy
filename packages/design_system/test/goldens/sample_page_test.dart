// SPDX-License-Identifier: Apache-2.0

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

// Test strings only; interface texts come from packages/l10n (M4-06).
const _title = 'Release notes';
const _body = 'Alex asked Sam and Robin to check the build before Friday.';
const _code = r'$ make check';
const _primary = 'Merge';
const _secondary = 'Cancel';

/// The sample page of the golden: a header, a paragraph, a code line, a primary and a
/// secondary button and a raised surface, all from the theme and the tokens.
class SamplePage extends StatelessWidget {
  const SamplePage({super.key});

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final text = Theme.of(context).textTheme;
    return SizedBox(
      width: DsLayout.overlayWidth,
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(_title, style: text.titleLarge),
            const SizedBox(height: DsSpace.s8),
            Text(_body, style: text.bodyMedium),
            const SizedBox(height: DsSpace.s8),
            Text(
              _code,
              style: dsTextStyle(DsType.code).copyWith(color: colors.codeFg),
            ),
            const SizedBox(height: DsSpace.s16),
            Row(
              children: [
                FilledButton(onPressed: () {}, child: const Text(_primary)),
                const SizedBox(width: DsSpace.s8),
                OutlinedButton(onPressed: () {}, child: const Text(_secondary)),
              ],
            ),
            const SizedBox(height: DsSpace.s16),
            DecoratedBox(
              decoration: BoxDecoration(
                color: colors.surfaceRaised,
                borderRadius: BorderRadius.circular(DsRadius.md),
                border: Border.all(
                  color: colors.borderSubtle,
                  width: DsBorderWidth.hairline,
                ),
                boxShadow: context.dsElevation.menu,
              ),
              child: SizedBox(
                width: double.infinity,
                height: DsSize.avatarLg,
                child: Center(child: Text(_title, style: text.labelLarge)),
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
    'sample page in light and dark, comfortable and touch, text scale 1 and 2',
    fileName: 'sample_page',
    builder: () => dsGoldenMatrix(
      name: 'sample page',
      builder: SamplePage.new,
      densities: const [DsDensityMode.comfortable, DsDensityMode.touch],
      textScales: const [1.0, 2.0],
    ),
  );
}
