// SPDX-License-Identifier: Apache-2.0

/// The paths of the app and their parser (docs/spec/15-client-architecture.md 11; brief M4-18
/// Design note 4; docs/DECISIONS.md D-M4-18-4).
library;

import 'package:domain/domain.dart';

/// The onboarding screens E1 to E7 (01 5.6) and their path segment below `/onboarding/`.
enum OnboardingStep {
  /// E1 Server address.
  server('server'),

  /// E2 Login.
  login('login'),

  /// E3 Redeem enrollment code.
  enroll('enroll'),

  /// E4 Device waiting for confirmation.
  pending('pending'),

  /// E5 Pair with another device.
  pair('pair'),

  /// E6 App lock.
  locked('locked'),

  /// E7 Logged out.
  signedOut('signed-out'),

  /// Not an onboarding screen: the app is signed in and shows the main view.
  signedIn('');

  const OnboardingStep(this.segment);

  /// The path segment.
  final String segment;

  /// The path of the screen, for example `/onboarding/login`.
  String get path => '/onboarding/$segment';
}

/// A target of the app, parsed from a path.
sealed class AppLocation {
  const AppLocation();

  /// Whether the location shows content of the instance and so needs a login (01 5.6: no
  /// content before the login).
  bool get needsLogin => true;
}

/// The main view without a selected target.
final class HomeLocation extends AppLocation {
  /// Creates the location.
  const HomeLocation();
}

/// `/onboarding/<step>`.
final class OnboardingLocation extends AppLocation {
  /// Creates the location.
  const OnboardingLocation(this.step);

  /// The screen.
  final OnboardingStep step;

  @override
  bool get needsLogin => false;
}

/// `/s/{space_id}`.
final class SpaceLocation extends AppLocation {
  /// Creates the location.
  const SpaceLocation(this.spaceId);

  /// The space.
  final SpaceId spaceId;
}

/// `/c/{channel_id}`.
final class ChannelLocation extends AppLocation {
  /// Creates the location.
  const ChannelLocation(this.channelId);

  /// The channel.
  final ChannelId channelId;
}

/// `/t/{thread_id}?m={message_id}`.
final class ThreadLocation extends AppLocation {
  /// Creates the location.
  const ThreadLocation(this.threadId, {this.messageId});

  /// The thread.
  final ThreadId threadId;

  /// The message to jump to, if any.
  final MessageId? messageId;
}

/// `/running`, `/mentions`, `/search?q=`: the cross-cutting views.
final class ViewLocation extends AppLocation {
  /// Creates the location.
  const ViewLocation(this.view, {this.query});

  /// `running`, `mentions` or `search`.
  final String view;

  /// The search text of `/search`.
  final String? query;
}

/// `/admin/...` and `/settings/...`: the sections M4-25 and the administration fill.
final class SectionLocation extends AppLocation {
  /// Creates the location.
  const SectionLocation(this.area, this.segments);

  /// `admin` or `settings`.
  final String area;

  /// The segments below the area.
  final List<String> segments;
}

/// A path that names no target, or an ID that is no valid ID: "Not found or no access"
/// (01 6.7).
final class UnknownLocation extends AppLocation {
  /// Creates the location.
  const UnknownLocation();
}

/// Parses [location] (path and query, as `GoRouterState.uri` gives it) into an [AppLocation].
///
/// A pure function: every path of 15 11 gives its location, everything else
/// [UnknownLocation]; IDs are checked as UUIDv7 by the ID types of `domain`. The same parser
/// serves links in messages, the push and links from outside (after
/// `pathOfExternalLink`).
///
/// Example: `parseLocation(Uri.parse('/t/<id>?m=<id>'))` is a [ThreadLocation].
AppLocation parseLocation(Uri location) {
  final segments = location.pathSegments.where((s) => s.isNotEmpty).toList();
  if (segments.isEmpty) return const HomeLocation();
  T? id<T>(T Function(String) parse, String value) {
    try {
      return parse(value);
    } on FormatException {
      return null;
    }
  }

  final rest = segments.sublist(1);
  switch (segments.first) {
    case 'onboarding' when rest.length == 1:
      for (final step in OnboardingStep.values) {
        if (step != OnboardingStep.signedIn && step.segment == rest.single) {
          return OnboardingLocation(step);
        }
      }
    case 's' when rest.length == 1:
      final space = id(SpaceId.parse, rest.single);
      if (space != null) return SpaceLocation(space);
    case 'c' when rest.length == 1:
      final channel = id(ChannelId.parse, rest.single);
      if (channel != null) return ChannelLocation(channel);
    case 't' when rest.length == 1:
      final thread = id(ThreadId.parse, rest.single);
      final m = location.queryParameters['m'];
      final message = m == null ? null : id(MessageId.parse, m);
      if (thread != null && (m == null || message != null)) {
        return ThreadLocation(thread, messageId: message);
      }
    case 'running' || 'mentions' when rest.isEmpty:
      return ViewLocation(segments.first);
    case 'search' when rest.isEmpty:
      return ViewLocation('search', query: location.queryParameters['q'] ?? '');
    case 'admin' || 'settings':
      return SectionLocation(segments.first, rest);
  }
  return const UnknownLocation();
}

/// Where the router sends a request for [location] (Design note 4).
///
/// [step] is the screen the onboarding controller shows. Signed out, every location that needs
/// a login goes to that screen and is kept in [keep] so it opens after the login (a link that
/// arrives before the login); an onboarding path of another screen goes to the current one.
/// Signed in, an onboarding path goes to [takeKept] (the kept link) or `/`. Returns null to
/// stay.
String? redirectFor(
  Uri location,
  OnboardingStep step, {
  required void Function(String path) keep,
  required String? Function() takeKept,
}) {
  final parsed = parseLocation(location);
  if (step != OnboardingStep.signedIn) {
    if (parsed is OnboardingLocation && parsed.step == step) return null;
    if (parsed.needsLogin && parsed is! HomeLocation) keep(location.toString());
    return step.path;
  }
  if (parsed is OnboardingLocation) return takeKept() ?? '/';
  return null;
}
