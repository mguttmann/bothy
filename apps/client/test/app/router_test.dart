// SPDX-License-Identifier: Apache-2.0

// The route parser and the redirect of the router (brief M4-18 Design note 4; 15 11; 01 6.7).

import 'package:client/app/deep_links.dart';
import 'package:client/app/router.dart';
import 'package:flutter_test/flutter_test.dart';

const String thread = '0193f2a7-0000-7000-8000-000000000001';
const String message = '0193f2a7-0000-7000-8000-000000000002';
const String space = '0193f2a7-0000-7000-8000-000000000003';
const String channel = '0193f2a7-0000-7000-8000-000000000004';

AppLocation parse(String path) => parseLocation(Uri.parse(path));

void main() {
  group('parseLocation', () {
    test('every path of 15 11 gives its location', () {
      expect(parse('/'), isA<HomeLocation>());
      expect((parse('/s/$space') as SpaceLocation).spaceId.value, space);
      expect((parse('/c/$channel') as ChannelLocation).channelId.value, channel);
      final t = parse('/t/$thread?m=$message') as ThreadLocation;
      expect(t.threadId.value, thread);
      expect(t.messageId?.value, message);
      expect((parse('/t/$thread') as ThreadLocation).messageId, isNull);
      expect((parse('/running') as ViewLocation).view, 'running');
      expect((parse('/mentions') as ViewLocation).view, 'mentions');
      final search = parse('/search?q=deploy') as ViewLocation;
      expect((search.view, search.query), ('search', 'deploy'));
      final admin = parse('/admin/people/x') as SectionLocation;
      expect(admin.area, 'admin');
      expect(admin.segments, ['people', 'x']);
      expect((parse('/settings') as SectionLocation).area, 'settings');
    });

    test('the onboarding screens have one path each', () {
      for (final step in OnboardingStep.values) {
        if (step == OnboardingStep.signedIn) continue;
        expect((parse(step.path) as OnboardingLocation).step, step);
        expect(parse(step.path).needsLogin, isFalse);
      }
      expect(parse('/onboarding/nothing'), isA<UnknownLocation>());
    });

    test('an unknown path or a malformed ID names no target', () {
      for (final path in [
        '/x',
        '/t/not-an-id',
        '/t/$thread?m=bad',
        '/s/$space/extra',
        '/running/extra',
      ]) {
        expect(parse(path), isA<UnknownLocation>(), reason: path);
      }
    });
  });

  group('external links', () {
    test('the host form and the path form give the app path', () {
      expect(
        pathOfExternalLink(
          Uri.parse('chat-example://t/$thread?m=$message'),
          'chat-example',
        ),
        '/t/$thread?m=$message',
      );
      expect(
        pathOfExternalLink(Uri.parse('Chat-Example:/c/$channel'), 'chat-example'),
        '/c/$channel',
      );
      expect(
        pathOfExternalLink(Uri.parse('chat-example://s/$space'), 'chat-example'),
        '/s/$space',
      );
    });

    test('another scheme or an empty link gives nothing', () {
      expect(
        pathOfExternalLink(Uri.parse('https://t/$thread'), 'chat-example'),
        isNull,
      );
      expect(pathOfExternalLink(Uri.parse('chat-example:'), 'chat-example'), isNull);
    });
  });

  group('redirectFor', () {
    String? kept;
    String? go(String path, OnboardingStep step) => redirectFor(
      Uri.parse(path),
      step,
      keep: (p) => kept = p,
      takeKept: () {
        final k = kept;
        kept = null;
        return k;
      },
    );

    setUp(() => kept = null);

    test('signed out, a content path goes to the current screen and is kept', () {
      expect(go('/t/$thread?m=$message', OnboardingStep.login), '/onboarding/login');
      expect(kept, '/t/$thread?m=$message');
    });

    test('a link before the login opens after it', () {
      go('/c/$channel', OnboardingStep.server);
      expect(go('/onboarding/server', OnboardingStep.signedIn), '/c/$channel');
      expect(go('/onboarding/login', OnboardingStep.signedIn), '/');
    });

    test('the current onboarding screen stays, another one is replaced', () {
      expect(go('/onboarding/pending', OnboardingStep.pending), isNull);
      expect(go('/onboarding/login', OnboardingStep.pending), '/onboarding/pending');
      expect(kept, isNull);
    });

    test('signed in, a content path stays', () {
      expect(go('/t/$thread', OnboardingStep.signedIn), isNull);
      expect(go('/x', OnboardingStep.signedIn), isNull);
    });
  });
}
