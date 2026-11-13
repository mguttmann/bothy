// SPDX-License-Identifier: Apache-2.0

// What a person typed in a composer stays there across a switch to another chat and back
// (docs/decisions.d/FIX-COMPOSER-DRAFT-LOST.md; 04 14). The app runs as it ships, as the
// desktop app and as the phone app, against the Dart fake server on 127.0.0.1: the composer
// writes the draft through the outbox, the fake keeps and deletes drafts as the server does
// (the draft that follows a send or an emptied field is a new row with a new ID), and its
// events reach the app over the gateway.
//
// The person never waits for the network: between typing, sending and switching the tests let
// no request finish, as a person does who sends a message and types the next one at once.

import 'dart:async';
import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../phone/support/phone_app.dart' show phoneWindow;
import '../support/fake_instance.dart';
import '../support/until.dart';
import '../support/widgets.dart';

/// The app of one test: the fake instance, the running app and the two chats of a forum.
class _App {
  _App(this.tester, this.fake, {required this.phone});

  final WidgetTester tester;
  final FakeInstance fake;

  /// Whether this is the phone app.
  final bool phone;

  ProviderContainer? container;

  /// The forum and two of its chats.
  late final String forum;
  late final String one;
  late final String two;

  LocalDatabase get db =>
      container!.read(onboardingControllerProvider).services!.database;

  Finder get _shell => find.byKey(
    phone ? const ValueKey('phone.shell') : const ValueKey('column.sidebar'),
  );

  GoRouter get _router => GoRouter.of(tester.element(_shell));

  /// The text field of the composer of [thread].
  Finder fieldOf(String thread) => find.descendant(
    of: find.byKey(ValueKey('historyColumn.$thread.null')),
    matching: find.descendant(
      of: find.byKey(const ValueKey('composer')),
      matching: find.byType(TextField),
    ),
  );

  /// The text field of the composer of the new chat view.
  Finder get newChatField => find.descendant(
    of: find.byKey(const ValueKey('newChat.composer')),
    matching: find.byType(TextField),
  );

  /// Runs [action] where the widgets live (the fake time of the test) while both clocks
  /// advance, so it shares the database with the streams the widgets watch.
  Future<T> inApp<T>(String what, Future<T> Function() action) async {
    var done = false;
    late T result;
    Object? error;
    unawaited(
      Future(() async {
        try {
          result = await action();
        } on Object catch (e) {
          error = e;
        } finally {
          done = true;
        }
      }),
    );
    final watch = Stopwatch()..start();
    while (!done) {
      if (watch.elapsed >= testPatience) fail('$what did not end');
      await tester.pump(const Duration(milliseconds: 1));
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 1)),
      );
    }
    if (error != null) throw error!;
    return result;
  }

  /// Lets real IO run and the frames advance for a moment.
  Future<void> _round() async {
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump(const Duration(milliseconds: 5));
  }

  /// Lets real IO run and the frames advance until [condition] holds on screen.
  Future<void> until(String Function() what, bool Function() condition) async {
    final watch = Stopwatch()..start();
    while (!condition()) {
      if (watch.elapsed >= testPatience) {
        fail('${what()} did not happen within $testPatience');
      }
      await _round();
    }
  }

  /// Like [until] for a [condition] that reads the database of the app.
  Future<void> untilStored(
    String what,
    Future<bool> Function() condition,
  ) async {
    final watch = Stopwatch()..start();
    while (!await inApp(what, condition)) {
      if (watch.elapsed >= testPatience) {
        fail('$what did not happen within $testPatience');
      }
      await _round();
    }
  }

  /// Signs Alex in and shows the app.
  Future<void> start() async {
    if (phone) {
      phoneWindow(tester);
    } else {
      desktopWindow(tester);
    }
    final seed = fake.seed;
    final world = fake.server.world;
    forum = world.addForum(seed.space, 'deploys', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    one = world.addThread(forum, 'One', createdBy: seed.sam.id);
    two = world.addThread(forum, 'Two', createdBy: seed.sam.id);
    final c = container = appContainer(fake);
    await tester.runAsync(() async {
      final control = await connected(c, fake);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      await eventually(
        'live',
        () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
      );
    });
    await _show();
  }

  Future<void> _show() async {
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container!,
        child: const ClientApp(),
      ),
    );
    await until(
      () => 'the shell of the app',
      () => _shell.evaluate().isNotEmpty,
    );
  }

  /// The person quits the app and starts it again on the same device.
  Future<void> restart() async {
    await tester.pumpWidget(const SizedBox());
    final first = container!;
    await inApp('closing the app', () async {
      final closing = first
          .read(onboardingControllerProvider)
          .services!
          .close();
      first.dispose();
      await closing;
    });
    await tester.pump(const Duration(minutes: 5));
    final second = container = appContainer(fake);
    await tester.runAsync(() async {
      second.read(onboardingControllerProvider);
      await eventually(
        'the main view of the second start',
        () =>
            second.read(onboardingControllerProvider).step ==
            OnboardingStep.signedIn,
      );
    });
    await _show();
  }

  /// Opens the chat [thread]; done when its composer is the only one on screen.
  Future<void> open(String thread) async {
    _router.go('/t/$thread');
    await until(
      () => 'the composer of the chat alone on screen',
      () =>
          fieldOf(thread).evaluate().isNotEmpty &&
          find.byKey(const ValueKey('composer')).evaluate().length == 1 &&
          newChatField.evaluate().isEmpty,
    );
  }

  /// Opens the view of a new chat in the forum.
  Future<void> openNew() async {
    _router.go('/c/$forum/new');
    await until(
      () => 'the composer of the new chat alone on screen',
      () =>
          newChatField.evaluate().isNotEmpty &&
          find.byKey(const ValueKey('composer')).evaluate().isEmpty,
    );
  }

  /// The person types [text] into [field]: the field has the focus and holds [text].
  Future<void> type(Finder field, String text) async {
    await tester.enterText(field, text);
    await tester.pump();
  }

  /// The person sends what the composer of [thread] holds, with its send button. The button
  /// takes a text the app put into the field in the next frame, and the phone slides it in
  /// once the field holds text: frames only, so no request finishes meanwhile.
  Future<void> send(String thread) async {
    final button = find.descendant(
      of: find.byKey(ValueKey('historyColumn.$thread.null')),
      matching: find.byWidgetPredicate(
        (w) => w is DsTappable && w.label == en.composerSend,
      ),
    );
    await tester.pump();
    final watch = Stopwatch()..start();
    while (button.hitTestable().evaluate().isEmpty) {
      if (watch.elapsed >= testPatience) fail('the send button did not show');
      await tester.pump(const Duration(milliseconds: 16));
    }
    await tester.tap(button);
    await tester.pump();
  }

  String textOf(Finder field) =>
      tester.widget<TextField>(field).controller!.text;

  /// Waits until [field] holds [text].
  Future<void> shows(Finder field, String text) => until(
    () =>
        '"$text" in the composer, which holds '
        '${field.evaluate().isEmpty ? 'nothing on screen' : '"${textOf(field)}"'},',
    () => field.evaluate().isNotEmpty && textOf(field) == text,
  );

  /// The drafts this device stores, as text.
  Future<List<String>> stored() => inApp(
    'reading the stored drafts',
    () async => [for (final d in await db.personalDao.allDrafts()) d.contentMd],
  );

  /// Whether every write of the outbox was answered; a write the server refused for good
  /// stays in the table as failed.
  Future<bool> answered() async =>
      (await db.outboxDao.allEntries()).every((e) => e.state.isFinal);

  /// Whether the app applied every event of the fake.
  Future<bool> caughtUp() async =>
      (await db.syncStateDao.read()).cursor?.value == fake.server.world.headSeq;

  /// Gives the views the time to take what was stored, so a text that must not show had its
  /// chance to.
  Future<void> settle() async {
    for (var i = 0; i < 20; i++) {
      await _round();
    }
  }

  /// Waits until nothing is on its way: every write of the outbox is answered and the app
  /// applied every event of the fake; then [settle].
  Future<void> idle() async {
    await untilStored(
      'the outbox sent and every event applied',
      () async => await answered() && await caughtUp(),
    );
    await settle();
  }

  /// Ends the app inside the test zone, where its timers live.
  Future<void> end() async {
    await tester.pumpWidget(const SizedBox());
    container?.dispose();
    container = null;
    await tester.pump(const Duration(minutes: 5));
  }
}

void main() {
  for (final phone in [false, true]) {
    group(phone ? 'the phone app' : 'the desktop app', () {
      HttpOverrides? saved;
      late FakeInstance fake;

      setUp(() async {
        saved = HttpOverrides.current;
        HttpOverrides.global = null;
        fake = await FakeInstance.start(platform: phone ? 'android' : 'macos');
      });

      tearDown(() async {
        await fake.stop();
        HttpOverrides.global = saved;
      });

      /// A test with the running app; the app ends inside the test zone also when [body]
      /// fails, so a failure leaves nothing for the next test.
      void scenario(String name, Future<void> Function(_App app) body) {
        testWidgets(name, (tester) async {
          final app = _App(tester, fake, phone: phone);
          try {
            await app.start();
            await body(app);
            expect(fake.server.violations, isEmpty);
          } finally {
            await app.end();
          }
        });
      }

      scenario('a text typed in a chat is there after a switch to another '
          'chat and back, and each chat keeps its own', (app) async {
        final one = app.fieldOf(app.one);
        final two = app.fieldOf(app.two);
        await app.open(app.one);
        await app.type(one, 'Hello Sam');
        // At once: the draft write has not been answered.
        await app.open(app.two);
        expect(app.textOf(two), isEmpty);
        await app.type(two, 'Other chat');
        await app.open(app.one);
        await app.shows(one, 'Hello Sam');
        await app.open(app.two);
        await app.shows(two, 'Other chat');
        await app.idle();
        expect(app.textOf(two), 'Other chat');
        await app.open(app.one);
        await app.shows(one, 'Hello Sam');
        await app.idle();
        expect(app.textOf(one), 'Hello Sam');
        expect(
          await app.stored(),
          unorderedEquals(['Hello Sam', 'Other chat']),
        );
      });

      scenario('a quick double switch keeps the text', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'Copied from the other chat');
        // Away, back, away and back, each as soon as the chat shows.
        await app.open(app.two);
        await app.open(app.one);
        await app.open(app.two);
        await app.open(app.one);
        await app.shows(one, 'Copied from the other chat');
        await app.idle();
        expect(app.textOf(one), 'Copied from the other chat');
      });

      // The loss of the report. The server deleted the draft of the sent message; its
      // `draft.deleted` reached the device while the person typed the next message and was
      // not applied (04 14: no change is taken over while typing); the next draft is a new row
      // on the server, and the device kept both.
      scenario('the message typed after a send is there after a switch and '
          'back', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'Message one');
        await app.idle();
        await app.send(app.one);
        // The person goes on typing; the answer and the events of the send arrive meanwhile.
        await app.type(one, 'Message');
        await app.idle();
        await app.type(one, 'Message two');
        // At once: the last draft write has not been answered.
        await app.open(app.two);
        await app.open(app.one);
        await app.shows(one, 'Message two');
        await app.idle();
        expect(app.textOf(one), 'Message two');
        // Away and back once more, with everything stored.
        await app.open(app.two);
        await app.open(app.one);
        await app.shows(one, 'Message two');
        await app.idle();
        expect(app.textOf(one), 'Message two');
        expect(await app.stored(), ['Message two']);
      });

      // The server deletes an emptied draft too, and the next one is a new row again.
      scenario('a text typed after the field was emptied is there after a '
          'switch and back', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'First try');
        await app.idle();
        await app.type(one, '');
        await app.idle();
        await app.type(one, 'Second try');
        await app.open(app.two);
        await app.open(app.one);
        await app.shows(one, 'Second try');
        await app.idle();
        expect(app.textOf(one), 'Second try');
        expect(await app.stored(), ['Second try']);
      });

      scenario('a sent text never comes back: not after the send, not after '
          'the next send, not after a switch and back', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'Message one');
        await app.idle();
        // The events of the send arrive while the person types the next message.
        fake.server.faults.holdDispatches();
        await app.send(app.one);
        expect(app.textOf(one), isEmpty);
        await app.until(
          () => 'the first message on the server',
          () => fake.server.requestsOf('createMessage').isNotEmpty,
        );
        await app.type(one, 'Message two');
        fake.server.faults.releaseDispatches();
        await app.idle();
        expect(app.textOf(one), 'Message two');
        await app.send(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        await app.open(app.two);
        await app.open(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
      });

      /// The messages the fake holds.
      int messages() => fake.server.world.log
          .where((e) => e.type == 'message.created')
          .length;

      // The draft ends on the device with the send (FIX-CDL/9). The `draft.deleted` of the
      // first send is not applied while the person types, and the draft write of the second
      // message never goes out: the second send drops it while the first is on its way. The
      // server has no draft then and sends no second `draft.deleted`.
      scenario('a sent text never comes back when the next message is sent '
          'before the answer of the first send', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'Message one');
        await app.idle();
        expect(await app.stored(), ['Message one']);
        final base = messages();
        // The answer of the first send is slow, its events are not.
        fake.server.faults.delay(
          const Duration(seconds: 3),
          operationId: 'createMessage',
        );
        await app.send(app.one);
        await app.type(one, 'Message two');
        await app.until(
          () => 'the first message on the server',
          () => messages() == base + 1,
        );
        await app.untilStored('the events of the first send', app.caughtUp);
        await app.send(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        await app.open(app.two);
        await app.open(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
      });

      // Three short messages in a row on a slow connection: every answer takes a while, and
      // the events of a send arrive with its answer.
      scenario('a sent text never comes back after three quick sends on a '
          'slow connection', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'Message one');
        await app.idle();
        expect(await app.stored(), ['Message one']);
        final base = messages();
        fake.server.faults
          ..delay(
            const Duration(seconds: 3),
            operationId: 'createMessage',
            times: 2,
          )
          ..holdDispatches();
        await app.send(app.one);
        await app.type(one, 'B');
        await app.send(app.one);
        // The first send is answered, the second is on its way.
        await app.until(
          () => 'the second message on the server',
          () => messages() == base + 2,
        );
        await app.type(one, 'C');
        fake.server.faults.releaseDispatches();
        await app.untilStored('the events of the sends', app.caughtUp);
        await app.send(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        await app.open(app.two);
        await app.open(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
      });

      // The draft of the next message never reaches the server: its write is refused for
      // good, so no later draft takes the place of the sent one.
      scenario('a sent text never comes back when the draft of the next '
          'message is refused', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'Message one');
        await app.idle();
        final base = messages();
        fake.server.faults.holdDispatches();
        await app.send(app.one);
        await app.until(
          () => 'the first message on the server',
          () => messages() == base + 1,
        );
        fake.server.faults.onRequest(
          'putThreadDraft',
          status: 422,
          code: 'validation_failed',
        );
        await app.type(one, 'Message two');
        fake.server.faults.releaseDispatches();
        await app.idle();
        await app.open(app.two);
        await app.open(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
      });

      // FIX-CDL/6: a row an earlier version of the app left, which the server no longer has.
      // The server deletes nothing with the send then and sends no `draft.deleted`.
      scenario('a text an earlier version of the app left in a chat is gone '
          'for good once it is sent', (app) async {
        final one = app.fieldOf(app.one);
        await app.inApp(
          'storing the row',
          () => app.db.personalDao.upsertDraftIfNewer(
            Draft(
              id: DraftId.parse(fake.server.world.nextId()),
              channelId: ChannelId.parse(app.forum),
              threadId: ThreadId.parse(app.one),
              contentMd: 'Left behind',
              fileIds: const [],
              version: const ResourceVersion(2),
            ),
          ),
        );
        await app.open(app.one);
        await app.shows(one, 'Left behind');
        await app.send(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        await app.open(app.two);
        await app.open(app.one);
        await app.idle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
      });

      // The draft write of the sent text is on its way when the person sends: its answer
      // arrives after the send, and the events of the send arrive later still.
      scenario('a sent text never comes back when the answer of its draft '
          'write arrives after the send', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        final base = messages();
        fake.server.faults
          ..delay(const Duration(seconds: 3), operationId: 'putThreadDraft')
          ..holdDispatches();
        await app.type(one, 'Message one');
        await app.untilStored('the draft write on its way', () async {
          final entries = await app.db.outboxDao.allEntries();
          return entries.any((e) => e.state == OutboxState.sending);
        });
        await app.send(app.one);
        await app.until(
          () => 'the message on the server',
          () => messages() == base + 1,
        );
        await app.untilStored('the answer of the send', app.answered);
        await app.settle();
        expect(app.textOf(one), isEmpty);
        await app.open(app.two);
        await app.open(app.one);
        await app.settle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
        fake.server.faults.releaseDispatches();
        await app.idle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
      });

      // 04 13: the send waits in the outbox, as it does without a connection. The draft
      // ended with the send, also before the server knows of it. The first attempt of the
      // send is refused for a while; `composer_test.dart` holds the same without a clock, with
      // an engine that never sends.
      scenario('a sent text is not back in the composer while its send '
          'waits', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'Message one');
        await app.idle();
        expect(await app.stored(), ['Message one']);
        fake.server.faults.onRequest(
          'createMessage',
          status: 503,
          code: 'unavailable',
          retryAfter: const Duration(seconds: 3),
        );
        await app.send(app.one);
        await app.open(app.two);
        await app.open(app.one);
        await app.settle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
        // The send goes out with the next attempt.
        await app.idle();
        expect(app.textOf(one), isEmpty);
        expect(await app.stored(), isEmpty);
      });

      scenario('the text of a new chat is there after a switch to a chat and '
          'back', (app) async {
        await app.openNew();
        await app.type(app.newChatField, 'An idea');
        await app.open(app.two);
        expect(app.textOf(app.fieldOf(app.two)), isEmpty);
        await app.openNew();
        await app.shows(app.newChatField, 'An idea');
        await app.idle();
        expect(app.textOf(app.newChatField), 'An idea');
      });

      scenario('the text of a new chat typed after the field was emptied is '
          'there after a switch and back', (app) async {
        await app.openNew();
        await app.type(app.newChatField, 'An idea');
        await app.idle();
        await app.type(app.newChatField, '');
        await app.idle();
        await app.type(app.newChatField, 'A better idea');
        await app.open(app.two);
        await app.openNew();
        await app.shows(app.newChatField, 'A better idea');
        await app.idle();
        expect(app.textOf(app.newChatField), 'A better idea');
        expect(await app.stored(), ['A better idea']);
      });

      // 15 6.2 and 6.3: the outbox and the drafts hold unsent work and stay with the device.
      scenario('a text typed right before the app quits is there after the '
          'next start', (app) async {
        final one = app.fieldOf(app.one);
        await app.open(app.one);
        await app.type(one, 'Typed before quitting');
        await app.restart();
        await app.open(app.one);
        await app.shows(one, 'Typed before quitting');
        await app.idle();
        expect(app.textOf(one), 'Typed before quitting');
        expect(await app.stored(), ['Typed before quitting']);
      });
    });
  }
}
