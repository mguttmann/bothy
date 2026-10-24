// SPDX-License-Identifier: Apache-2.0

// The notation of the search field (brief M4-23 Design note 2, docs/spec/14-search.md 4.1):
// `from:`, `in:`, `has:file`, `before:` and `after:` become parameters, unknown handles and
// channels stay text, and the server never sees the notation.

import 'package:client/search/query_notation.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';

final List<Principal> principals = [
  Principal(
    id: alex,
    kind: PrincipalKind.user,
    handle: 'alex',
    displayName: 'Alex',
  ),
  Principal(
    id: sam,
    kind: PrincipalKind.user,
    handle: 'sam',
    displayName: 'Sam',
  ),
  Principal(
    id: robin,
    kind: PrincipalKind.agent,
    handle: 'dev-runner',
    displayName: 'Robin',
  ),
];

final List<Channel> channels = [
  channel(1, name: 'general'),
  channel(2, name: 'releases'),
  channel(3, spaceN: 2, name: 'releases'),
  channel(4, name: 'release notes'),
];

QueryNotation parse(String input) =>
    parseQueryNotation(input, principals: principals, channels: channels);

void main() {
  test('plain text stays the free text', () {
    final n = parse('resume window');
    expect(n.text, 'resume window');
    expect(n.authorId, isNull);
    expect(n.channelIds, isEmpty);
    expect(n.hasFile, isNull);
    expect(n.after, isNull);
    expect(n.before, isNull);
  });

  test('from: resolves a stored handle to author_id, with or without @', () {
    expect(parse('from:sam deploy').authorId, sam);
    expect(parse('from:sam deploy').text, 'deploy');
    expect(parse('deploy from:@Dev-Runner').authorId, robin);
    expect(parse('deploy from:@Dev-Runner').text, 'deploy');
  });

  test('an unknown handle stays text', () {
    final n = parse('from:casey deploy');
    expect(n.authorId, isNull);
    expect(n.text, 'from:casey deploy');
  });

  test('in: resolves every visible channel of that name', () {
    final n = parse('in:releases version');
    expect(n.channelIds, [channelId(2), channelId(3)]);
    expect(n.text, 'version');
    expect(parse('in:#General x').channelIds, [channelId(1)]);
  });

  test('in: with a quoted name and an unknown channel', () {
    final quoted = parse('in:"release notes" draft');
    expect(quoted.channelIds, [channelId(4)]);
    expect(quoted.text, 'draft');
    final unknown = parse('in:random draft');
    expect(unknown.channelIds, isEmpty);
    expect(unknown.text, 'in:random draft');
  });

  test('has:file sets has_file; another has: value stays text', () {
    expect(parse('has:file invoice').hasFile, isTrue);
    expect(parse('has:file invoice').text, 'invoice');
    final other = parse('has:link invoice');
    expect(other.hasFile, isNull);
    expect(other.text, 'has:link invoice');
  });

  test('before: and after: are UTC day starts; an invalid date stays text', () {
    final n = parse('after:2026-10-01 before:2026-10-15 report');
    expect(n.after, DateTime.utc(2026, 10, 1));
    expect(n.before, DateTime.utc(2026, 10, 15));
    expect(n.text, 'report');
    final bad = parse('before:2026-13-40 report');
    expect(bad.before, isNull);
    expect(bad.text, 'before:2026-13-40 report');
    expect(parse('after:yesterday x').text, 'after:yesterday x');
  });

  test('mixed text keeps phrases, operators and the order of the rest', () {
    final n = parse(
      '"resume window" from:alex -draft in:general or has:file heartbeat',
    );
    expect(n.text, '"resume window" -draft or heartbeat');
    expect(n.authorId, alex);
    expect(n.channelIds, [channelId(1)]);
    expect(n.hasFile, isTrue);
  });

  test('notation inside a quoted phrase stays text', () {
    final n = parse('"from:alex in:general"');
    expect(n.authorId, isNull);
    expect(n.channelIds, isEmpty);
    expect(n.text, '"from:alex in:general"');
  });

  test('the last from: wins and several in: add up', () {
    final n = parse('from:alex from:sam in:general in:releases x');
    expect(n.authorId, sam);
    expect(n.channelIds, [channelId(1), channelId(2), channelId(3)]);
    expect(n.text, 'x');
  });

  test('a key without a value or an unknown key stays text', () {
    expect(parse('from: x').text, 'from: x');
    expect(parse('to:alex x').text, 'to:alex x');
    expect(parse('').text, '');
  });
}
