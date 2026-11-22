// SPDX-License-Identifier: Apache-2.0

// The team controller over a fake API (09 3.9.1, 3.9.7) and the client command `/team`.

import 'package:client/team/team_command.dart';
import 'package:client/team/team_controller.dart';
import 'package:client/team/team_model.dart';
import 'package:client/team/team_port.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/structure.dart';
import '../support/team_fakes.dart';

TeamController controllerOf(FakeTeamPort port, {List<String>? changes}) {
  var keys = 0;
  return TeamController(
    channel: channelId(1),
    port: port,
    newKey: () => 'key-${++keys}',
    onChanged: () => changes?.add('changed'),
  );
}

void main() {
  test('load reads the team; a channel without team mode has none', () async {
    final none = controllerOf(FakeTeamPort());
    await none.load();
    expect(none.loaded, isTrue);
    expect(none.enabled, isFalse);

    final some = controllerOf(
      FakeTeamPort(team(members: [member(1, 'coding')])),
    );
    await some.load();
    expect(some.team!.members.single.handle, 'coding');
  });

  test('switching on and off calls the server and reports a change', () async {
    final port = FakeTeamPort();
    final changes = <String>[];
    final c = controllerOf(port, changes: changes);
    expect(await c.setEnabled(on: true), isTrue);
    expect(c.enabled, isTrue);
    expect(await c.setEnabled(on: false), isTrue);
    expect(c.enabled, isFalse);
    expect(port.calls, ['enable', 'disable']);
    expect(changes, ['changed', 'changed']);
  });

  test(
    'a refusal keeps the team and names the reason, no change reported',
    () async {
      final port = FakeTeamPort()
        ..failures.add(
          const TeamFailure.problem(
            'conflict',
            reason: 'team_readers_not_owners',
          ),
        );
      final changes = <String>[];
      final c = controllerOf(port, changes: changes);
      expect(await c.setEnabled(on: true), isFalse);
      expect(c.enabled, isFalse);
      expect(c.failure!.reason, 'team_readers_not_owners');
      expect(changes, isEmpty);
      expect(c.busy, isFalse);
    },
  );

  test('add repeats its idempotency key after a lost answer, a new one after success', () async {
    final port = FakeTeamPort(team())
      ..failures.add(const TeamFailure.offline());
    final c = controllerOf(port);
    await c.load();
    final args = (agent: robin, home: channelId(2), handle: 'coding');
    expect(
      await c.add(agent: args.agent, home: args.home, handle: args.handle),
      isFalse,
    );
    expect(c.failure!.offline, isTrue);
    expect(
      await c.add(agent: args.agent, home: args.home, handle: args.handle),
      isTrue,
    );
    expect(c.team!.members.single.handle, 'coding');
    expect(await c.add(agent: nova, home: channelId(3), handle: 'qa'), isTrue);
    expect(port.calls, [
      'read',
      'add coding key-1',
      'add coding key-1',
      'add qa key-2',
    ]);
  });

  test('another handle after a refusal is a new request', () async {
    final port = FakeTeamPort(team());
    final c = controllerOf(port);
    await c.load();
    port.failures.add(
      const TeamFailure.problem('conflict', reason: 'team_handle_taken'),
    );
    await c.add(agent: robin, home: channelId(2), handle: 'coding');
    await c.add(agent: robin, home: channelId(2), handle: 'coder');
    expect(port.calls.skip(1), ['add coding key-1', 'add coder key-2']);
  });

  test('the same add after a refusal gets a new key, the server stored the refusal', () async {
    final port = FakeTeamPort(team());
    final c = controllerOf(port);
    await c.load();
    port.failures.add(
      const TeamFailure.problem('conflict', reason: 'team_handle_taken'),
    );
    await c.add(agent: robin, home: channelId(2), handle: 'qa');
    expect(await c.add(agent: robin, home: channelId(2), handle: 'qa'), isTrue);
    expect(port.calls.skip(1), ['add qa key-1', 'add qa key-2']);
  });

  test('remove drops the member; stop all pauses the team', () async {
    final port = FakeTeamPort(
      team(
        members: [
          member(1, 'coding'),
          member(2, 'qa', agent: nova),
        ],
      ),
    );
    final c = controllerOf(port);
    await c.load();
    expect(await c.remove(member(1, 'coding').id), isTrue);
    expect(c.team!.members.map((m) => m.handle), ['qa']);
    expect(await c.stopAll(), isTrue);
    expect(c.team!.pause, TeamPause.stopped);
    expect(port.calls.last, 'stop key-1');
  });

  test('one operation at a time', () async {
    final port = FakeTeamPort(team());
    final c = controllerOf(port);
    final first = c.stopAll();
    expect(c.busy, isTrue);
    expect(
      await c.stopAll(),
      isFalse,
      reason: 'the second press waits for nothing',
    );
    await first;
    expect(port.calls, ['stop key-1']);
  });

  test('an answer after dispose does not throw', () async {
    final port = FakeTeamPort(team());
    final c = controllerOf(port);
    final pending = c.load();
    c.dispose();
    await pending;
  });

  group('/team', () {
    late AppLocalizations l;
    setUpAll(() async {
      l = await AppLocalizations.delegate.load(const Locale('en'));
    });

    Future<(List<String>, List<String?>)> runCommand(
      FakeTeamPort port,
      String argument,
    ) async {
      final said = <String>[];
      final dialogs = <String?>[];
      var keys = 0;
      await runTeamCommand(
        parseTeamCommand(argument),
        channel: channelId(1),
        port: port,
        newKey: () => 'key-${++keys}',
        l: l,
        openDialog: ({handle}) async => dialogs.add(handle),
        say: said.add,
      );
      return (said, dialogs);
    }

    test('list and add open the dialog, add with its handle', () async {
      final port = FakeTeamPort(team());
      expect((await runCommand(port, 'list')).$2, [null]);
      expect((await runCommand(port, 'add @coding')).$2, ['coding']);
      expect(port.calls, isEmpty);
    });

    test('remove takes the member by handle', () async {
      final port = FakeTeamPort(team(members: [member(1, 'coding')]));
      final (said, _) = await runCommand(port, 'remove @Coding');
      expect(port.calls, ['read', 'remove ${member(1, 'coding').id}']);
      expect(said, [l.teamRemoved('coding')]);
    });

    test('remove of an unknown handle says so and removes nobody', () async {
      final port = FakeTeamPort(team(members: [member(1, 'coding')]));
      final (said, _) = await runCommand(port, 'remove qa');
      expect(port.calls, ['read']);
      expect(said, [l.teamUnknownHandle('qa')]);
    });

    test('stop stops every member run', () async {
      final port = FakeTeamPort(team(members: [member(1, 'coding')]));
      final (said, _) = await runCommand(port, 'stop');
      expect(port.calls, ['read', 'stop key-1']);
      expect(said, [l.teamStopped]);
    });

    test(
      'remove and stop in a channel without team mode change nothing',
      () async {
        final port = FakeTeamPort();
        expect((await runCommand(port, 'stop')).$1, [l.teamNotEnabled]);
        expect((await runCommand(port, 'remove @qa')).$1, [l.teamNotEnabled]);
        expect(port.calls, ['read', 'read']);
      },
    );

    test('a refused stop names the reason', () async {
      final port = FakeTeamPort(team())
        ..failures.addAll([const TeamFailure.offline()]);
      final (said, _) = await runCommand(port, 'stop');
      expect(said, [l.teamOffline]);
    });

    test('a failed read changes nothing and says why', () async {
      final port = FakeTeamPort(team(members: [member(1, 'coding')]))
        ..readFailure = const TeamFailure.offline();
      expect((await runCommand(port, 'remove @coding')).$1, [l.teamOffline]);
      expect(port.calls, ['read']);
    });

    test('an unknown sub-command shows the usage', () async {
      final port = FakeTeamPort(team());
      expect((await runCommand(port, 'allow @a @b')).$1, [l.teamUsage]);
      expect(port.calls, isEmpty);
    });
  });
}
