// SPDX-License-Identifier: Apache-2.0

/// The tabs and navigation stacks of the phone (docs/spec/01-product.md 5.3 and 4;
/// docs/spec/15-client-architecture.md 11; brief M5-02 Design note 2; docs/decisions.d/
/// M5-02.md D-M5-02-3).
///
/// Pure functions: the router keeps the paths of 15 11, and these functions decide which tab
/// a path belongs to and which pages its tab shows below it. The phone shell keeps one
/// navigator per tab and the last path of each tab, so each tab keeps its stack.
library;

import 'package:domain/domain.dart';

import '../app/router.dart';

/// The tabs of the bottom tab bar, in their order (01 5.3 M1).
enum PhoneTab {
  /// Spaces, then channel or post list, then history.
  spaces('/'),

  /// The mentions of the person (01 D5).
  mentions('/mentions'),

  /// Running now (01 D10).
  running('/running'),

  /// The person: settings, devices and sessions, logout.
  me('/settings');

  const PhoneTab(this.root);

  /// The path of the tab's first page.
  final String root;
}

/// The kind of one page of a tab's stack.
enum PhonePageKind {
  /// M1 Spaces.
  spaces,

  /// A channel: the post list of a forum (M2) or the main history of a text channel.
  channel,

  /// The history of a thread (M3).
  thread,

  /// Mentions.
  mentions,

  /// Running now.
  running,

  /// Search.
  search,

  /// The Me page.
  me,

  /// One section of the settings.
  settingsSection,

  /// "Not found or no access" (01 6.7).
  notFound,
}

/// One page of a tab's stack: its kind and the path that shows it on top.
final class PhonePage {
  /// Creates the page.
  const PhonePage(this.kind, this.path);

  /// What the page shows.
  final PhonePageKind kind;

  /// The router path whose stack has this page on top; going back to the page goes to it.
  final String path;

  @override
  bool operator ==(Object other) =>
      other is PhonePage && other.kind == kind && other.path == path;

  @override
  int get hashCode => Object.hash(kind, path);

  @override
  String toString() => 'PhonePage(${kind.name}, $path)';
}

/// The tab of [location] when [current] is the active tab.
///
/// The root path of a tab selects that tab. A thread stays in Mentions or Running when it was
/// opened there; [external] (a deep link or a push, 15 11) and every other thread, channel,
/// space or search path go to Spaces. Settings and administration paths belong to Me. An
/// unknown path stays in [current].
///
/// Example: `tabOf(parseLocation(Uri.parse('/t/<id>')), PhoneTab.mentions)` is
/// [PhoneTab.mentions].
PhoneTab tabOf(
  AppLocation location,
  PhoneTab current, {
  bool external = false,
}) {
  switch (location) {
    case HomeLocation():
      return PhoneTab.spaces;
    case ViewLocation(view: 'mentions'):
      return PhoneTab.mentions;
    case ViewLocation(view: 'running'):
      return PhoneTab.running;
    // The phone layout of the run graph is M12 (M5-14); its path belongs to Running.
    case RunGraphLocation():
      return PhoneTab.running;
    case SectionLocation():
      return PhoneTab.me;
    case ThreadLocation():
      if (external) return PhoneTab.spaces;
      return current == PhoneTab.mentions || current == PhoneTab.running
          ? current
          : PhoneTab.spaces;
    case SpaceLocation() || ChannelLocation() || ViewLocation():
      return PhoneTab.spaces;
    case UnknownLocation() || OnboardingLocation():
      return current;
  }
}

/// What a thread's stack needs to know about its channel: the channel, if stored, and its
/// main thread (a text channel), so the stack is spaces, channel, thread (brief M5-02 Design
/// note 2), and a main thread is not shown twice.
typedef ThreadChannel = ({ChannelId channel, ThreadId? mainThread});

/// The pages of [tab] for [location], bottom first; the last page is on top.
///
/// [channelOf] names the channel of a thread location once it is stored (null before: the
/// stack is spaces, thread, and the channel page joins when the thread arrives).
///
/// Example: in Spaces, `/t/<t>` of a forum `<c>` gives the pages Spaces (`/`), channel
/// (`/c/<c>`) and thread (`/t/<t>`).
List<PhonePage> phoneStack(
  PhoneTab tab,
  Uri location, {
  ThreadChannel? channelOf,
}) {
  final path = location.toString();
  final parsed = parseLocation(location);
  final root = switch (tab) {
    PhoneTab.spaces => const PhonePage(PhonePageKind.spaces, '/'),
    PhoneTab.mentions => const PhonePage(PhonePageKind.mentions, '/mentions'),
    PhoneTab.running => const PhonePage(PhonePageKind.running, '/running'),
    PhoneTab.me => const PhonePage(PhonePageKind.me, '/settings'),
  };
  if (path == tab.root) return [root];
  switch (parsed) {
    case HomeLocation() || SpaceLocation():
      return [root];
    case ViewLocation(view: 'mentions' || 'running'):
      return [root];
    case ChannelLocation():
      return [root, PhonePage(PhonePageKind.channel, path)];
    case ThreadLocation(:final threadId):
      final channel = channelOf;
      return [
        root,
        if (tab == PhoneTab.spaces &&
            channel != null &&
            channel.mainThread != threadId)
          PhonePage(PhonePageKind.channel, '/c/${channel.channel.value}'),
        PhonePage(PhonePageKind.thread, path),
      ];
    case ViewLocation(view: 'search'):
      return [root, PhonePage(PhonePageKind.search, path)];
    case SectionLocation(area: 'settings', :final segments)
        when segments.length == 1:
      return [root, PhonePage(PhonePageKind.settingsSection, path)];
    case SectionLocation(area: 'settings', :final segments)
        when segments.isEmpty:
      return [root];
    default:
      return [root, PhonePage(PhonePageKind.notFound, path)];
  }
}
