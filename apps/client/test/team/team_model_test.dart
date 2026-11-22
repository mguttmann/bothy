// SPDX-License-Identifier: Apache-2.0

// What the app derives from a team (09 3.9): member states, message labels, the mentioned
// handles, the parsing of `/team`, the home channel choices and the suggested handle.

import 'package:client/team/providers.dart' show teamCapable;
import 'package:client/team/team_model.dart';
import 'package:client/team/team_port.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import '../support/team_fakes.dart';
import '../support/thread_fixtures.dart';

void main() {
  group('member state', () {
    final coding = member(1, 'coding');

    test('an active run of the member agent in its work thread is running', () {
      for (final state in [
        RunState.queued,
        RunState.running,
        RunState.waitingInput,
      ]) {
        expect(
          teamMemberState(coding, [run(1, state, threadN: 2)], null),
          TeamMemberState.running,
        );
      }
    });

    test('a running member stays running while the channel is paused', () {
      expect(
        teamMemberState(coding, [
          run(1, RunState.running, threadN: 2),
        ], TeamPause.stopped),
        TeamMemberState.running,
      );
    });

    test('without an active run: idle, or paused while the channel is', () {
      final ended = [run(1, RunState.completed, threadN: 2)];
      expect(teamMemberState(coding, ended, null), TeamMemberState.idle);
      expect(
        teamMemberState(coding, ended, TeamPause.pingpong),
        TeamMemberState.paused,
      );
    });

    test('a run of another agent or another thread does not count', () {
      final other = member(2, 'qa', agent: nova);
      expect(
        teamMemberState(other, [run(1, RunState.running, threadN: 2)], null),
        TeamMemberState.idle,
        reason: 'the run is Robin\'s, the member is Nova',
      );
      expect(
        teamMemberState(coding, [run(1, RunState.running, threadN: 3)], null),
        TeamMemberState.idle,
      );
    });
  });

  group('message labels', () {
    final t = team(
      members: [
        member(1, 'coding'),
        member(2, 'qa', agent: nova),
      ],
    );

    test('a text reply of a member agent is from that member', () {
      final label = teamMessageLabel(
        t,
        msg(1, author: robin, origin: MessageOrigin.agent),
      );
      expect(label, isA<FromMember>());
      expect((label! as FromMember).handles, ['coding']);
    });

    test('an agent that is a member twice names both handles', () {
      final twice = t.withMember(
        member(3, 'coding-docs', homeN: 3, threadN: 4),
      );
      final label = teamMessageLabel(
        twice,
        msg(1, author: robin, origin: MessageOrigin.agent),
      );
      expect((label! as FromMember).handles, ['coding', 'coding-docs']);
    });

    test(
      'a mirror with its team handle gets no line, its member is the author',
      () {
        final twice = t.withMember(
          member(3, 'coding-docs', homeN: 3, threadN: 4),
        );
        final label = teamMessageLabel(
          twice,
          msg(
            1,
            author: robin,
            origin: MessageOrigin.agent,
          ).copyWith(teamHandle: 'coding-docs'),
        );
        expect(label, isNull);
      },
    );

    test('an owner text message without a member mention goes to all', () {
      expect(teamMessageLabel(t, msg(1, author: alex)), isA<ToAllMembers>());
      expect(
        teamMessageLabel(t, msg(2, author: alex, text: 'ask @nobody')),
        isA<ToAllMembers>(),
        reason: 'a handle of no member narrows nothing',
      );
      expect(
        teamMessageLabel(t, msg(3, author: alex, text: 'run `@qa` now')),
        isA<ToAllMembers>(),
        reason: 'a mention in a code span is no mention',
      );
    });

    test('an owner text message that mentions members goes to them', () {
      final label = teamMessageLabel(
        t,
        msg(1, author: alex, text: '@QA check this, then @nobody and @coding'),
      );
      expect(label, isA<ToMembers>());
      expect((label! as ToMembers).handles, [
        'qa',
        'coding',
      ], reason: 'the members in the order of their first mention');
    });

    test(
      'no label for a non-member agent, a note, a voice or deleted message',
      () {
        final other = PrincipalId.parse(uuid(0xa9));
        expect(
          teamMessageLabel(
            t,
            msg(1, author: other, origin: MessageOrigin.agent),
          ),
          isNull,
        );
        expect(
          teamMessageLabel(
            t,
            msg(2, origin: MessageOrigin.system, kind: MessageKind.system),
          ),
          isNull,
        );
        expect(
          teamMessageLabel(t, msg(3, author: alex, kind: MessageKind.voice)),
          isNull,
        );
        expect(
          teamMessageLabel(t, msg(4, author: alex, deleted: true)),
          isNull,
        );
      },
    );
  });

  group('mentioned handles (09 3.9.4, Go TestMentionedHandles)', () {
    for (final (name, text, want) in [
      ('none', 'no mention here', <String>[]),
      ('one', '@review I built X, please take over', ['review']),
      (
        'order and duplicates',
        '@qa and @Review, then @review again and @qa',
        ['qa', 'review'],
      ),
      ('punctuation', '(@coding), @qa: done. @ops!', ['coding', 'qa', 'ops']),
      ('email is no mention', 'mail alex@example.com or @@qa', <String>[]),
      ('bare at', '@ and @ alone', <String>[]),
      ('code span', 'run `@coding fix` then @qa', ['qa']),
      ('double backtick span', 'see ``a ` @coding`` and @qa', ['qa']),
      ('unclosed backtick', 'a ` @coding', ['coding']),
      (
        'fenced block',
        'before @qa\n```\n@coding inside\n```\nafter @ops',
        ['qa', 'ops'],
      ),
      ('tilde fence', '~~~go\n@coding\n~~~\n@qa', ['qa']),
      ('unclosed fence hides the rest', '@qa\n```\n@coding', ['qa']),
    ]) {
      test(name, () => expect(mentionedHandles(text), want));
    }
  });

  group('/team', () {
    test('list is the default', () {
      expect(parseTeamCommand(''), isA<TeamList>());
      expect(parseTeamCommand('  LIST '), isA<TeamList>());
    });

    test('add with and without a handle', () {
      expect((parseTeamCommand('add') as TeamAdd).handle, isNull);
      expect((parseTeamCommand('add @Coding') as TeamAdd).handle, 'coding');
    });

    test('remove needs a handle', () {
      expect((parseTeamCommand('remove @qa') as TeamRemove).handle, 'qa');
      expect((parseTeamCommand('Remove qa') as TeamRemove).handle, 'qa');
      expect(parseTeamCommand('remove'), isA<TeamUsage>());
      expect(parseTeamCommand('remove @'), isA<TeamUsage>());
    });

    test('stop, and everything else is the usage', () {
      expect(parseTeamCommand('stop'), isA<TeamStop>());
      expect(parseTeamCommand('stop now'), isA<TeamUsage>());
      expect(parseTeamCommand('allow @a @b'), isA<TeamUsage>());
    });
  });

  test(
    'handles: lower-case letters, digits, - and _, from a letter or digit',
    () {
      for (final ok in ['coding', 'qa-2', 'a_b', '9x']) {
        expect(teamHandlePattern.hasMatch(ok), isTrue, reason: ok);
      }
      for (final bad in ['', '-a', '_a', 'Coding', 'a b', '@a', 'é']) {
        expect(teamHandlePattern.hasMatch(bad), isFalse, reason: bad);
      }
    },
  );

  test('home channels: operated by the agent, no DM, archived, read-only or the team', () {
    Channel ch(int n, {ChannelKind kind = ChannelKind.text, PrincipalId? op}) =>
        channel(n, kind: kind, operator: op ?? robin);
    final archived = Channel(
      id: channelId(7),
      kind: ChannelKind.text,
      name: 'old',
      topic: '',
      sortKey: 'a7',
      operator: ChannelOperator(
        mode: OperatorMode.agent,
        agentId: robin,
        effectiveAgentId: robin,
      ),
      readOnly: false,
      dmParticipantIds: const [],
      createdAt: t0,
      archivedAt: t0,
      version: const ResourceVersion(1),
      settingsVersion: const ResourceVersion(1),
    );
    final homes = teamHomeCandidates(
      [
        ch(1),
        ch(2, kind: ChannelKind.forum),
        ch(3),
        ch(4, op: nova),
        ch(5, kind: ChannelKind.dm),
        archived,
      ],
      robin,
      team: channelId(1),
    );
    expect(homes.map((c) => c.id), [channelId(2), channelId(3)]);
  });

  test('home channels: a pair that is already a member is not offered', () {
    final chans = [
      channel(1),
      channel(2, operator: robin),
      channel(3, operator: robin),
    ];
    final t = team(members: [member(1, 'coding')]);
    expect(teamFreeHomeCandidates(chans, robin, t).map((c) => c.id), [
      channelId(3),
    ], reason: 'Robin in channel 2 is the member coding');
    expect(
      teamFreeHomeCandidates(chans, nova, t),
      isEmpty,
      reason: 'Nova operates nothing here',
    );
  });

  group('suggested handle', () {
    test('from a name: lower case, reduced to the handle pattern', () {
      expect(suggestTeamHandle('Website', taken: const []), 'website');
      expect(suggestTeamHandle('My API v2!', taken: const []), 'my-api-v2');
      expect(suggestTeamHandle('--Docs__', taken: const []), 'docs__');
      expect(suggestTeamHandle('Café Ops', taken: const []), 'caf-ops');
    });

    test('the fallback when the name leaves nothing, else empty', () {
      expect(
        suggestTeamHandle('!!!', fallback: 'Nova', taken: const []),
        'nova',
      );
      expect(suggestTeamHandle('', taken: const []), '');
    });

    test('unique among the members in any letter case', () {
      expect(suggestTeamHandle('qa', taken: const ['QA']), 'qa-2');
      expect(suggestTeamHandle('qa', taken: const ['qa', 'qa-2']), 'qa-3');
    });

    test('at most the default team.handle_max_chars', () {
      final h = suggestTeamHandle('a' * 40, taken: const []);
      expect(h.length, 32);
      expect(teamHandlePattern.hasMatch(h), isTrue);
      final unique = suggestTeamHandle('a' * 40, taken: ['a' * 32]);
      expect(unique, '${'a' * 30}-2');
    });
  });

  test('only an open text channel can be a team channel', () {
    expect(teamCapable(channel(1)), isTrue);
    expect(teamCapable(channel(1, kind: ChannelKind.forum)), isFalse);
    expect(teamCapable(channel(1, kind: ChannelKind.dm)), isFalse);
  });
}
