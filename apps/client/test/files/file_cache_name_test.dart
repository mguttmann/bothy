// SPDX-License-Identifier: Apache-2.0

// The last path segment of a cached file (brief M4-22; 12 9): the original name of another
// person becomes one valid segment on every desktop system.

import 'package:client/files/file_cache.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('a name of dots and spaces becomes file', () {
    for (final name in [' .. ', '..', '.', '   ', ' . . ']) {
      expect(FileCache.safeFileName(name), 'file', reason: '"$name"');
    }
  });

  test('leading dots go after trimming, trailing dots and spaces go too', () {
    expect(FileCache.safeFileName(' .hidden.txt '), 'hidden.txt');
    expect(FileCache.safeFileName('report.txt. .'), 'report.txt');
    expect(FileCache.safeFileName('a/b\\c.txt'), 'a_b_c.txt');
  });

  test('Windows device names get a prefix, with or without an extension', () {
    expect(FileCache.safeFileName('CON'), '_CON');
    expect(FileCache.safeFileName('con.txt'), '_con.txt');
    expect(FileCache.safeFileName('Lpt1.tar.gz'), '_Lpt1.tar.gz');
    expect(FileCache.safeFileName('console.txt'), 'console.txt');
  });
}
