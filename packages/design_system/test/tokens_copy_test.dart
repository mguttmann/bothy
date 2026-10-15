// SPDX-License-Identifier: Apache-2.0

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('the package copy of the generator output exists', () {
    // flutter test runs with the package folder as the working directory.
    final copy = File('lib/src/generated/design_tokens.g.dart');
    expect(copy.existsSync(), isTrue);
    expect(
      copy.readAsStringSync(),
      contains('final class DsColors extends ThemeExtension<DsColors>'),
    );
  });

  test('DsColors has an instance per theme', () {
    expect(DsColors.light, isNot(DsColors.dark));
    expect(DsColors.light.textPrimary, isNot(DsColors.dark.textPrimary));
  });

  test('DsAccent has an instance per accent and brightness', () {
    const accents = [
      'spruce',
      'loch',
      'heather',
      'rowan',
      'amber',
      'gorse',
      'moss',
    ];
    for (final brightness in Brightness.values) {
      final variants = {
        for (final key in accents) DsAccent.of(key, brightness),
      };
      expect(variants, hasLength(accents.length), reason: '$brightness');
    }
    expect(DsAccent.of('spruce', Brightness.light), DsAccent.lightSpruce);
    expect(DsAccent.of('moss', Brightness.dark), DsAccent.darkMoss);
  });

  test('DsDensity has an instance per density', () {
    final heights = {
      DsDensity.comfortable.controlHeight,
      DsDensity.compact.controlHeight,
      DsDensity.touch.controlHeight,
    };
    expect(heights, hasLength(3));
  });

  test('DsMotion has an instance per motion context', () {
    expect(DsMotion.full.durationFade, greaterThan(Duration.zero));
    expect(DsMotion.reduced.durationMove, Duration.zero);
    expect(DsMotion.reduced, isNot(DsMotion.full));
  });

  test('DsElevation has an instance per theme', () {
    expect(DsElevation.light, isNot(DsElevation.dark));
  });
}
