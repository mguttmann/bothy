// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

DsContext _resolve({
  String? theme,
  String? density,
  String? reduceMotion,
  Brightness platformBrightness = Brightness.light,
  bool platformReduceMotion = false,
  bool isTouchPlatform = false,
  String? accent,
}) {
  return resolveDsContext(
    appearanceTheme: theme,
    appearanceDensity: density,
    appearanceReduceMotion: reduceMotion,
    platformBrightness: platformBrightness,
    platformReduceMotion: platformReduceMotion,
    isTouchPlatform: isTouchPlatform,
    accentKey: accent,
  );
}

void main() {
  group('appearance.theme', () {
    test('system with a dark platform gives dark', () {
      expect(
        _resolve(theme: 'system', platformBrightness: Brightness.dark).theme,
        DsThemeMode.dark,
      );
    });
    test('system with a light platform gives light', () {
      expect(_resolve(theme: 'system').theme, DsThemeMode.light);
    });
    test('absent follows the platform', () {
      expect(
        _resolve(platformBrightness: Brightness.dark).theme,
        DsThemeMode.dark,
      );
    });
    test('light overrides a dark platform', () {
      expect(
        _resolve(theme: 'light', platformBrightness: Brightness.dark).theme,
        DsThemeMode.light,
      );
    });
    test('dark overrides a light platform', () {
      expect(_resolve(theme: 'dark').theme, DsThemeMode.dark);
    });
    test('an unknown value follows the platform (default system)', () {
      expect(
        _resolve(theme: 'neon', platformBrightness: Brightness.dark).theme,
        DsThemeMode.dark,
      );
      expect(_resolve(theme: 'neon').theme, DsThemeMode.light);
    });
  });

  group('appearance.density', () {
    test('a touch platform with compact gives touch', () {
      expect(
        _resolve(density: 'compact', isTouchPlatform: true).density,
        DsDensityMode.touch,
      );
    });
    test('a touch platform without a value gives touch', () {
      expect(_resolve(isTouchPlatform: true).density, DsDensityMode.touch);
    });
    test('desktop without a value gives comfortable', () {
      expect(_resolve().density, DsDensityMode.comfortable);
    });
    test('desktop with compact gives compact', () {
      expect(_resolve(density: 'compact').density, DsDensityMode.compact);
    });
    test('desktop with comfortable gives comfortable', () {
      expect(
        _resolve(density: 'comfortable').density,
        DsDensityMode.comfortable,
      );
    });
    test('an unknown value gives comfortable', () {
      expect(_resolve(density: 'tiny').density, DsDensityMode.comfortable);
      expect(_resolve(density: 'touch').density, DsDensityMode.comfortable);
    });
  });

  group('appearance.reduce_motion', () {
    for (final platform in [false, true]) {
      test('on gives reduced (platform $platform)', () {
        expect(
          _resolve(reduceMotion: 'on', platformReduceMotion: platform).motion,
          DsMotionMode.reduced,
        );
      });
      test('off gives full (platform $platform)', () {
        expect(
          _resolve(reduceMotion: 'off', platformReduceMotion: platform).motion,
          DsMotionMode.full,
        );
      });
      final expected = platform ? DsMotionMode.reduced : DsMotionMode.full;
      test('system follows the platform ($platform)', () {
        expect(
          _resolve(
            reduceMotion: 'system',
            platformReduceMotion: platform,
          ).motion,
          expected,
        );
      });
      test('absent follows the platform ($platform)', () {
        expect(_resolve(platformReduceMotion: platform).motion, expected);
      });
      test('an unknown value follows the platform ($platform)', () {
        expect(
          _resolve(
            reduceMotion: 'sometimes',
            platformReduceMotion: platform,
          ).motion,
          expected,
        );
      });
    }
  });

  group('accent', () {
    test('a known accent stays', () {
      expect(_resolve(accent: 'loch').accent, 'loch');
      expect(_resolve(accent: 'moss').accent, 'moss');
    });
    test('the default accent stays', () {
      expect(_resolve(accent: 'spruce').accent, 'spruce');
    });
    test('an unknown accent gives spruce', () {
      expect(_resolve(accent: 'purple').accent, 'spruce');
    });
    test('an absent accent gives spruce', () {
      expect(_resolve().accent, dsDefaultAccent);
    });
  });

  group('DsContext', () {
    const a = DsContext(
      theme: DsThemeMode.dark,
      density: DsDensityMode.compact,
      motion: DsMotionMode.reduced,
      accent: 'loch',
    );
    test('is a value type', () {
      expect(a.copyWith(), a);
      expect(a.copyWith().hashCode, a.hashCode);
      expect(a.copyWith(accent: 'moss'), isNot(a));
      expect(a.copyWith(theme: DsThemeMode.light).theme, DsThemeMode.light);
      expect(
        a.copyWith(density: DsDensityMode.touch).density,
        DsDensityMode.touch,
      );
      expect(a.copyWith(motion: DsMotionMode.full).motion, DsMotionMode.full);
      expect(a.toString(), 'DsContext(dark, compact, reduced, loch)');
    });
  });
}
