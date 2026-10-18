// SPDX-License-Identifier: Apache-2.0

/// Builders of the test world with the example people Alex, Sam and Robin (brief M4-17 Scope;
/// white label: only `example.com` names and these people).
library;

import 'world.dart';

/// The standard world of the scenarios.
///
/// - Alex and Sam: people with local accounts (password and TOTP below); Alex owns the
///   instance.
/// - Robin: an agent.
/// - Space "Team" with the channels `general` (Alex `trigger`, Sam `write`, Robin `write`),
///   `support` (Alex and Sam `write`) and `private` (only Sam, so Alex's catch-up skips its
///   events and gets `cursor` frames).
/// - Threads: "Release notes" in `general`, "Printer" in `support`, "Budget" in `private`.
///
/// Example:
/// ```dart
/// final server = await FakeServer.start();
/// final w = WorldSeed.standard(server.world);
/// await auth.login.loginWithPassword(
///   loginName: WorldSeed.alexLogin,
///   password: WorldSeed.password,
///   totpCode: WorldSeed.totp,
/// );
/// ```
final class WorldSeed {
  WorldSeed._(this.world);

  /// The login name of Alex.
  static const String alexLogin = 'alex';

  /// The login name of Sam.
  static const String samLogin = 'sam';

  /// The password of both local accounts.
  static const String password = 'correct-horse-battery';

  /// The TOTP code the fake accepts for both.
  static const String totp = '123456';

  /// The world.
  final FakeWorld world;

  /// Alex, the person of the client under test.
  late final FakePerson alex;

  /// Sam, another person.
  late final FakePerson sam;

  /// Robin, an agent.
  late final FakePerson robin;

  /// The space "Team".
  late final String space;

  /// The channel `general`.
  late final String general;

  /// The channel `support`.
  late final String support;

  /// The channel `private`, which Alex does not see.
  late final String private;

  /// The thread "Release notes" in `general`.
  late final String releaseNotes;

  /// The thread "Printer" in `support`.
  late final String printer;

  /// The thread "Budget" in `private`.
  late final String budget;

  /// Fills [world] with the standard world and returns the IDs.
  static WorldSeed standard(FakeWorld world) {
    final s = WorldSeed._(world);
    s.alex = world.addPerson(
      handle: 'alex',
      displayName: 'Alex',
      loginName: alexLogin,
      password: password,
      totpCode: totp,
      isOwner: true,
    );
    s.sam = world.addPerson(
      handle: 'sam',
      displayName: 'Sam',
      loginName: samLogin,
      password: password,
      totpCode: totp,
    );
    s.robin = world.addPerson(
      handle: 'robin',
      displayName: 'Robin',
      kind: 'agent',
    );
    s.space = world.addSpace('Team');
    s.general = world.addChannel(s.space, 'general', {
      s.alex.id: 'trigger',
      s.sam.id: 'write',
      s.robin.id: 'write',
    });
    s.support = world.addChannel(s.space, 'support', {
      s.alex.id: 'write',
      s.sam.id: 'write',
    });
    s.private = world.addChannel(s.space, 'private', {s.sam.id: 'write'});
    s.releaseNotes = world.addThread(
      s.general,
      'Release notes',
      createdBy: s.alex.id,
    );
    s.printer = world.addThread(s.support, 'Printer', createdBy: s.sam.id);
    s.budget = world.addThread(s.private, 'Budget', createdBy: s.sam.id);
    return s;
  }

  /// Posts [text] as [author] into [threadId] through the write path of the world (as if the
  /// author's client sent it) and returns the stored message.
  JsonMap post(FakePerson author, String threadId, String text) =>
      world.createMessage(author.id, threadId, {
        'client_msg_id': world.nextId(),
        'content_md': text,
      }).message;
}
