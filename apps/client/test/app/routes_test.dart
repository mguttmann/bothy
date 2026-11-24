// SPDX-License-Identifier: Apache-2.0

// The routes of the app and the parser hold the same paths (AUD-PKG-P02-client-b): every path
// `parseLocation` knows matches a route on the phone and on the desktop, so only an unknown
// path goes through the error builder.

import 'package:client/app/app.dart';
import 'package:client/app/router.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

const String _id = '0193f2a7-0000-7000-8000-000000000001';
const String _other = '0193f2a7-0000-7000-8000-000000000002';

/// One path of each form the parser knows (15 11).
const List<String> knownPaths = [
  '/',
  '/s/$_id',
  '/c/$_id',
  '/c/$_id/new',
  '/t/$_id?m=$_other',
  '/running',
  '/running/$_id?t=$_other',
  '/mentions',
  '/live',
  '/search?q=deploy',
  '/hosts',
  '/hosts/$_id',
  '/status',
  '/settings',
  '/settings/appearance',
  '/admin',
  '/admin/people',
  '/admin/people/$_id',
  '/onboarding/server',
];

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('the known paths are the ones the parser reads', () {
    for (final path in knownPaths) {
      expect(
        parseLocation(Uri.parse(path)),
        isNot(isA<UnknownLocation>()),
        reason: path,
      );
    }
  });

  for (final phone in [true, false]) {
    test('every path the parser knows has a route '
        '(${phone ? 'phone' : 'desktop'})', () {
      final router = GoRouter(routes: appRoutes(phone: phone));
      addTearDown(router.dispose);
      for (final path in [...knownPaths, '/share']) {
        final match = router.configuration.findMatch(Uri.parse(path));
        expect(match.isError, isFalse, reason: path);
      }
      expect(
        router.configuration.findMatch(Uri.parse('/nowhere')).isError,
        isTrue,
      );
    });
  }
}
