// SPDX-License-Identifier: Apache-2.0

// Name suggestion and the checks of the new project dialog (01 6.9 item 2, D-M4-43-3,
// D-M4-43-10).

import 'package:client/new_project/project_name.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const max = 40;

  test('a name from the last segment of the URL without .git', () {
    expect(
      suggestProjectName(
        url: 'https://git.example.com/team/Web-Site.git',
        maxChars: max,
      ),
      'web-site',
    );
    expect(
      suggestProjectName(
        url: 'https://git.example.com/team/website/',
        maxChars: max,
      ),
      'website',
    );
    expect(
      suggestProjectName(url: 'https://git.example.com', maxChars: max),
      '',
    );
    expect(suggestProjectName(url: 'not a url', maxChars: max), 'not-a-url');
  });

  test('a name from the first words of the idea', () {
    expect(
      suggestProjectName(
        idea: 'A shopping list app that syncs between phones.\nWith dark mode.',
        maxChars: max,
      ),
      'a-shopping-list-app',
    );
    expect(
      suggestProjectName(idea: 'Größe über alles', maxChars: max),
      'groesse-ueber-alles',
    );
    expect(
      suggestProjectName(idea: r'`rm -rf` $(x) "y"', maxChars: max),
      'rm-rf-x-y',
    );
    expect(suggestProjectName(idea: '   ', maxChars: max), '');
  });

  test('the suggestion is cut at the limit without a trailing separator', () {
    final name = suggestProjectName(
      idea: 'abcdefghij klmnopqrst uvwxyz',
      maxChars: 12,
    );
    expect(name, 'abcdefghij');
    expect(isProjectName(name, maxChars: 12), isTrue);
  });

  test('project names: one segment, starts with a letter or digit, within the limit', () {
    for (final ok in ['website', 'Web_site.2', 'a', '9-lives']) {
      expect(isProjectName(ok, maxChars: max), isTrue, reason: ok);
    }
    for (final bad in [
      '',
      '.hidden',
      '-x',
      '_x',
      'a/b',
      'a b',
      'wébsite',
      'a' * 41,
    ]) {
      expect(isProjectName(bad, maxChars: max), isFalse, reason: bad);
    }
  });

  test('clone URLs: https with a host, no user, query or fragment', () {
    expect(isCloneUrl('https://git.example.com/team/website.git'), isTrue);
    for (final bad in [
      '',
      'http://git.example.com/a',
      'ssh://git@git.example.com/a',
      'git@git.example.com:team/a.git',
      'https://alex:secret@git.example.com/a',
      'https://git.example.com/a?x=1',
      'https://git.example.com/a#x',
      'https:///a',
      'https://git.example.com/a b',
      '-https://git.example.com/a',
    ]) {
      expect(isCloneUrl(bad), isFalse, reason: bad);
    }
  });

  test('existing folders: relative under the projects folder', () {
    for (final ok in ['website', 'team/website', 'web site']) {
      expect(isFolderRef(ok), isTrue, reason: ok);
    }
    for (final bad in [
      '',
      '/srv/x',
      '~/x',
      '-x',
      r'a\b',
      'C:x',
      'a/../b',
      '..',
      ' x',
    ]) {
      expect(isFolderRef(bad), isFalse, reason: bad);
    }
  });
}
