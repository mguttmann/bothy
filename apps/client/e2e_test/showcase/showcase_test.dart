// SPDX-License-Identifier: Apache-2.0

// The showcase of the apps (brief M7-28 Design notes 1 and 2; docs/spec/01-product.md 5): the
// real app against the demo instance of `make demo` signs Alex in with the demo's enrollment
// code, opens the views the README shows in every theme the runner asks for and captures each as
// a PNG through Flutter's image API (showcase_capture.dart): spaces, the thread with the running
// live box, the approval card, Live with the topology, the hosts and a terminal on the demo host;
// on the desktop also TV mode. With `video` it then plays the one flow of the video (thread, live
// box, approval, terminal, live topology) while tools/showcase/capture.py records the emulator's
// screen. The images go to the runner through the report data of `flutter drive`
// (test_driver/showcase_driver.dart).
//
// tools/showcase/capture.py runs it with the define SHOWCASE; it lives outside
// integration_test/, so the platform jobs without a demo never pick it up (D-W-121), and without
// the define it fails at once (P-09: never skipped).

import 'dart:convert';
import 'dart:io';

import 'package:client/settings/settings_providers.dart';
import 'package:client/terminal/providers.dart';
import 'package:design_system/design_system.dart' show DsButton;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../support/e2e_app.dart';
import '../support/e2e_network.dart';
import 'showcase_capture.dart';
import 'showcase_target.dart';

/// The names of the data set (infra/demo/demo.yaml) the showcase opens.
abstract final class DemoNames {
  /// The space of the two open runs.
  static const String space = 'Engineering';

  /// The channel of the watcher run with its live box.
  static const String builds = 'builds';

  /// The thread of the watcher run.
  static const String watchThread = 'Watch the nightly build';

  /// The channel of the reviewer run with its approval card.
  static const String releases = 'releases';

  /// The thread of the reviewer run.
  static const String approvalThread = 'Publish the 2.4 release notes';

  /// The host of the terminal.
  static const String host = 'Demo host';
}

/// The command the terminal image shows: made-up text only, nothing of the build host.
const String terminalCommand = 'echo "Hello from the demo host"';

/// The line the video flow prints when the app waits for the recording, followed by the path
/// of the file whose existence starts the flow.
const String videoReadyMarker = 'showcase: video ready ';

/// The line the video flow prints when it is done.
const String videoDoneMarker = 'showcase: video done';

/// The IDs of the demo the showcase opens, read through the public API of the session.
final class DemoIds {
  DemoIds._(
    this.space,
    this.watchThread,
    this.approvalThread,
    this.host,
    this.builds,
  );

  /// The space of the two open runs.
  final String space;

  /// The thread of the watcher run.
  final String watchThread;

  /// The thread of the reviewer run.
  final String approvalThread;

  /// The demo host.
  final String host;

  /// The channel `builds`.
  final String builds;

  /// Reads the IDs through [app]'s session; throws a [StateError] naming what is missing.
  static Future<DemoIds> read(E2eApp app) async {
    final api = app.state.services!.auth.api;
    final spaces = (await api.getSpacesApi().listSpaces()).data!.items;
    final space = spaces.firstWhere(
      (s) => s.space.name == DemoNames.space,
      orElse: () => throw StateError('no space ${DemoNames.space}'),
    );
    final channels = (await api.getChannelsApi().listSpaceChannels(
      spaceId: space.space.id,
    )).data!.items;
    String channel(String name) => channels
        .firstWhere(
          (c) => c.channel.name == name,
          orElse: () => throw StateError('no channel $name'),
        )
        .channel
        .id;
    Future<String> thread(String channelId, String title) async {
      final page = (await api.getThreadsApi().listChannelThreads(
        channelId: channelId,
        limit: 100,
      )).data!.items;
      // The newest thread of that title: the live prompt of the demo.
      final hits = page.where((t) => t.title == title).toList()
        ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
      if (hits.isEmpty) throw StateError('no thread "$title"');
      return hits.first.id;
    }

    final builds = channel(DemoNames.builds);
    final hosts = (await api.getHostsApi().listHosts()).data!.items;
    final host = hosts.firstWhere(
      (h) => h.name == DemoNames.host,
      orElse: () => throw StateError('no host ${DemoNames.host}'),
    );
    return DemoIds._(
      space.space.id,
      await thread(builds, DemoNames.watchThread),
      await thread(channel(DemoNames.releases), DemoNames.approvalThread),
      host.id,
      builds,
    );
  }
}

/// One run of the showcase: the app, the IDs and the captured images.
final class ShowcaseRun {
  ShowcaseRun(this.app, this.target, this.ids);

  /// The app.
  final E2eApp app;

  /// The demo.
  final ShowcaseTarget target;

  /// The IDs of the demo.
  final DemoIds ids;

  /// The captured images by name (`<view>-<theme>`), as base64 of the PNG.
  final Map<String, String> images = {};

  WidgetTester get _tester => app.tester;

  /// Opens [path] in the app's router.
  Future<void> go(String path) async {
    GoRouter.of(_tester.element(find.byType(Navigator).first)).go(path);
    await _tester.pump();
  }

  /// Sets the theme of the person and waits until the app draws it.
  Future<void> setTheme(String theme) async {
    await _tester.runAsync(
      () => app.container
          .read(settingsRepositoryProvider)!
          .set(SettingKeys.theme, theme),
    );
    final want = theme == 'dark' ? Brightness.dark : Brightness.light;
    await app.until(
      'the $theme theme',
      () =>
          Theme.of(_tester.element(find.byType(Navigator).first)).brightness ==
          want,
    );
  }

  /// Captures the current screen as `<view>-<theme>`.
  Future<void> capture(String view, String theme) async {
    final png = await capturePng(_tester, width: target.captureWidth);
    images['$view-$theme'] = base64.encode(png);
  }

  /// The thread with the watcher's live box.
  Future<void> openWatchThread() async {
    await go('/t/${ids.watchThread}');
    await app.waitFor(find.byKey(ValueKey('history.${ids.watchThread}')));
    await app.waitFor(
      find.byWidgetPredicate(
        (w) =>
            w.key is ValueKey<String> &&
            (w.key! as ValueKey<String>).value.startsWith('livebox.line.'),
      ),
    );
  }

  /// The thread with the reviewer's approval card.
  Future<void> openApprovalThread() async {
    await go('/t/${ids.approvalThread}');
    await app.waitFor(find.byKey(ValueKey('history.${ids.approvalThread}')));
    await app.waitFor(find.byKey(const ValueKey('ds-component-approval.once')));
  }

  /// Live (D12 on the desktop, M10 on the phone) with its first view, the topology.
  Future<void> openLive() async {
    await go('/live');
    await app.waitFor(find.byType(Scrollable));
    // The first frame of the live stream and the queries need a moment on a live server; the
    // page is done once no progress indicator is left.
    await app.until(
      'the live page loaded',
      () => find.byType(CircularProgressIndicator).evaluate().isEmpty,
    );
  }

  /// The hosts (M9 on the phone, the hosts page on the desktop).
  Future<void> openHosts() async {
    await go('/hosts');
    await app.waitFor(find.textContaining(DemoNames.host));
  }

  /// A shell on the demo host with the step-up, then [terminalCommand]; returns when its output
  /// is on the screen. A shell opened before (the first theme) is shown again.
  Future<void> openTerminal() async {
    final open = app.container.read(terminalTabsProvider(ids.host));
    if (open?.current?.state.phase == TerminalPhase.connected) {
      await go('/hosts/${ids.host}');
      await app.waitFor(find.byType(TerminalScreenView));
      return;
    }
    if (target.phone) {
      await go('/hosts');
      await app.tap('phone.hosts.new.${ids.host}');
      await app.tapSettled(
        find.byKey(ValueKey('phone.hosts.new.${ids.host}.shell')),
      );
    } else {
      await go('/hosts/${ids.host}');
      await app.tap('terminal.new');
      await app.tapSettled(find.byKey(const ValueKey('terminal.new.shell')));
    }
    final confirm = find.text(
      AppLocalizations.of(_tester.element(find.byType(Navigator).first))
          .terminalOpenConfirm,
    );
    await app.until('the dialog "Open terminal"', () {
      return confirm.evaluate().isNotEmpty;
    });
    await app.tapSettled(confirm);
    final tabs = app.container.read(terminalTabsProvider(ids.host))!;
    bool connected() => tabs.current?.state.phase == TerminalPhase.connected;
    final dialog = find.byKey(const ValueKey('stepUp.continue'));
    await app.until(
      'the shell connected or the step-up',
      () => connected() || dialog.evaluate().isNotEmpty,
    );
    if (!connected()) {
      await app.tapSettled(dialog);
      await app.until('the shell connected', connected);
    }
    await _type(terminalCommand);
    await app.until(
      'the output of the command',
      () => tabs.current!.screen.visibleText().contains(
        'Hello from the demo host\n',
      ),
    );
  }

  Future<void> _type(String text) async {
    await _tester.tap(find.byType(TerminalScreenView));
    await _tester.pump();
    for (final c in '$text\n'.split('')) {
      final base = _tester.testTextInput.editingState!['text']! as String;
      _tester.testTextInput.updateEditingValue(
        TextEditingValue(
          text: '$base$c',
          selection: TextSelection.collapsed(offset: base.length + 1),
        ),
      );
      await _tester.pump();
    }
  }

  /// Every image of one theme.
  Future<void> captureTheme(String theme) async {
    await setTheme(theme);
    await go('/');
    await app.waitFor(find.byKey(ValueKey('sidebar.space.${ids.space}')));
    await capture('spaces', theme);
    await openWatchThread();
    await capture('thread-live', theme);
    await openApprovalThread();
    await capture('approval', theme);
    await openLive();
    await capture('live', theme);
    await openHosts();
    await capture('hosts', theme);
    await openTerminal();
    await capture('terminal', theme);
  }

  /// The flow of the video: waits until the runner records, then thread, live box, approval,
  /// terminal and live topology, each held for [hold] so a viewer can read it.
  Future<void> playVideo({Duration hold = const Duration(seconds: 3)}) async {
    final start = File('${Directory.systemTemp.path}/showcase-go');
    if (start.existsSync()) start.deleteSync();
    // The runner reads the marker from the output of flutter drive and creates the file once
    // the recording runs.
    // ignore: avoid_print
    print('$videoReadyMarker${start.path}');
    await app.until('the recording started', start.existsSync);
    Future<void> pause() => app.untilOr(() => false, hold);

    await go('/');
    await app.waitFor(find.byKey(ValueKey('sidebar.space.${ids.space}')));
    await pause();
    await openWatchThread();
    await pause();
    await openApprovalThread();
    await pause();
    await app.tap('ds-component-approval.once');
    // The card is decided once its buttons are gone or disabled (single use, 09 11.3).
    final approve = find.byKey(const ValueKey('ds-component-approval.once'));
    await app.until(
      'the decided approval card',
      () =>
          approve.evaluate().isEmpty ||
          (approve.evaluate().first.widget as DsButton).onPressed == null,
    );
    await pause();
    await openTerminal();
    await pause();
    await openLive();
    await pause();
    // ignore: avoid_print
    print(videoDoneMarker);
  }
}

void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  testWidgets('showcase of the app against the demo instance', (tester) async {
    final target = ShowcaseTarget.fromEnvironment();
    final network = await E2eNetwork.start(
      target.server,
      target.caPem,
      upstreamHost: target.upstreamAddress,
    );
    network.installForServer();
    if (!target.phone) {
      // The desktop window of the images: 1440 x 960 logical pixels at a Retina ratio.
      tester.view
        ..physicalSize = const Size(1440, 960) * 2
        ..devicePixelRatio = 2;
    }
    E2eApp? started;
    try {
      final app = started = await E2eApp.start(
        tester,
        target,
        platform: target.platform,
        deviceView: true,
        deviceName: target.phone ? 'Alex phone' : 'Alex laptop',
      );
      await app.connect();
      await app.enroll(target.code);
      await app.waitFor(
        find.byKey(ValueKey(target.phone ? 'phone.shell' : 'column.sidebar')),
      );
      await app.waitLive();
      final ids = (await tester.runAsync(() => DemoIds.read(app)))!;
      final run = ShowcaseRun(app, target, ids);
      for (final theme in target.themes) {
        await run.captureTheme(theme);
      }
      if (target.video) await run.playVideo();
      binding.reportData = {'images': run.images};
    } finally {
      await started?.stop();
      await network.stop();
      HttpOverrides.global = null;
    }
  });
}
