// SPDX-License-Identifier: Apache-2.0

// The counters and the replay of a catch-up (04 14, D-M4-19-10): the snapshot's
// `unread_threads` hold every event up to its `head_seq`. A catch-up from a stored cursor
// replays the events between that cursor and the head after `ready`; none of them may change a
// counter again, whatever its type (`message.created`, `message.updated`, `message.deleted`,
// `read_state.updated`). Live events after the head count as usual.
//
// A seeded randomized model test over the whole class: a random history of one thread, a
// random stored cursor and a random snapshot head; the counters after the catch-up must equal
// the server's exact values at the head, and after a live tail the server's values at its end.

import 'dart:math';

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

/// One message of the server's model.
final class _Msg {
  _Msg(this.seq, {required this.mine, required this.mention});
  final int seq;
  final bool mine;
  bool mention;
  bool deleted = false;
  int version = 1;
}

/// The server's exact state of thread 1 for Alex.
final class _Server {
  final Map<int, _Msg> msgs = {};
  int lastRead = 0;
  int readVersion = 1;
  int lastSeq = 0;

  Iterable<_Msg> get _unread =>
      msgs.values.where((m) => !m.deleted && !m.mine && m.seq > lastRead);

  ThreadCounts get counts => ThreadCounts(
    unread: _unread.length,
    mentions: _unread.where((m) => m.mention).length,
  );
}

Map<String, Object?> _created(int seq, _Msg m) => envelope(
  'message.created',
  seq: seq,
  payload: {
    'message': messageJson(
      1,
      m.seq,
      author: m.mine ? alexId : samId,
      mentions: m.mention ? [alexId] : const [],
    ),
  },
  resourceKind: 'message',
  resourceId: messageId(1, m.seq),
  version: 1,
  scope: {'channel_id': channelId(1), 'thread_id': threadId(1)},
);

Map<String, Object?> _updated(int seq, _Msg m) => envelope(
  'message.updated',
  seq: seq,
  payload: {
    'message': messageJson(
      1,
      m.seq,
      version: m.version,
      author: m.mine ? alexId : samId,
      mentions: m.mention ? [alexId] : const [],
      content: 'edited ${m.version}',
    ),
  },
  resourceKind: 'message',
  resourceId: messageId(1, m.seq),
  version: m.version,
  scope: {'channel_id': channelId(1), 'thread_id': threadId(1)},
);

Map<String, Object?> _deleted(int seq, _Msg m) => envelope(
  'message.deleted',
  seq: seq,
  payload: {
    'message_id': messageId(1, m.seq),
    'thread_id': threadId(1),
    'thread_seq': m.seq,
    'deleted_at': at,
    'deleted_by': samId,
  },
  resourceKind: 'message',
  resourceId: messageId(1, m.seq),
  version: m.version,
  scope: {'channel_id': channelId(1), 'thread_id': threadId(1)},
);

/// `read_state.updated` as the wire carries it: the computed mention count, no unread count
/// (openapi `ReadState`).
Map<String, Object?> _readState(int seq, _Server s) => envelope(
  'read_state.updated',
  seq: seq,
  payload: {
    'read_state': readStateJson(
      1,
      lastRead: s.lastRead,
      version: s.readVersion,
      mentions: s.counts.mentions,
      device: otherDevice,
    ),
  },
  resourceKind: 'thread',
  resourceId: threadId(1),
  personal: true,
);

/// Builds the next event of the history at global [seq] and applies it to [s]. In the live
/// tail ([live]) only new messages and reads happen; a live read reads at least everything up
/// to [headLastSeq], the last message the snapshot held (D-M4-19-10: the unread value of a
/// read state is computed from the stored `last_message_seq`, which only the snapshot's
/// counters cover).
Map<String, Object?> _next(
  Random r,
  _Server s,
  int seq, {
  required bool live,
  int headLastSeq = 0,
}) {
  final open = s.msgs.values.where((m) => !m.deleted).toList();
  final others = open.where((m) => !m.mine).toList();
  final roll = r.nextInt(100);
  if (!live && roll < 15 && others.isNotEmpty) {
    // An edit that adds or removes the mention of Alex.
    final m = others[r.nextInt(others.length)];
    m
      ..mention = !m.mention
      ..version += 1;
    return _updated(seq, m);
  }
  if (!live && roll < 30 && open.isNotEmpty) {
    final m = open[r.nextInt(open.length)]
      ..deleted = true
      ..version += 1;
    return _deleted(seq, m);
  }
  if (roll < 45 && s.lastSeq > s.lastRead) {
    final from = live && headLastSeq > s.lastRead
        ? headLastSeq
        : s.lastRead + 1;
    s
      ..lastRead = from >= s.lastSeq
          ? s.lastSeq
          : from + r.nextInt(s.lastSeq - from + 1)
      ..readVersion += 1;
    return _readState(seq, s);
  }
  final m = _Msg(++s.lastSeq, mine: r.nextInt(5) == 0, mention: r.nextBool());
  s.msgs[m.seq] = m;
  return _created(seq, m);
}

Map<String, Object?> _unreadOf(_Server s) => {
  'thread_id': threadId(1),
  'channel_id': channelId(1),
  'unread_count': s.counts.unread,
  'mention_count': s.counts.mentions,
  'last_message_seq': s.lastSeq,
  'last_read_seq': s.lastRead,
};

Future<void> _run(int seed) async {
  final r = Random(seed);
  final w = World();
  try {
    final t1 = ThreadId.parse(threadId(1));
    final s = _Server();
    final cursor =
        2 + r.nextInt(8); // seq 1 is the thread, then events up to the cursor
    final head = cursor + r.nextInt(12);
    final end = head + r.nextInt(6);
    final history = <int, Map<String, Object?>>{1: threadCreated(1, 1)};
    ThreadCounts? atHead;
    var headLastSeq = 0;
    Map<String, Object?>? snapshotUnread;
    for (var seq = 2; seq <= end; seq++) {
      history[seq] = _next(
        r,
        s,
        seq,
        live: seq > head,
        headLastSeq: headLastSeq,
      );
      if (seq == head) {
        headLastSeq = s.lastSeq;
        atHead = s.counts;
        snapshotUnread = _unreadOf(s);
      }
    }
    atHead ??= s.counts;
    w.rest.routes['GET /me/read-states'] = (_) =>
        const FakeReply(200, {'items': <Object?>[]});

    // First session: the device follows the history live up to its cursor.
    await w.connect(cursor: 0, headSeq: 0, snapshot: snapshotJson(headSeq: 0));
    await w.push('catchup.complete', {'head_seq': 0});
    await w.dispatch([for (var seq = 1; seq <= cursor; seq++) history[seq]!]);
    expect(await w.cursor(), cursor, reason: 'seed $seed: first session');

    // Offline until the head; the reconnect gets a new session with a snapshot.
    w.ch.serverClose(4000);
    await w.idle();
    await w.h.clock.elapse(const Duration(minutes: 2));
    await w.idle();
    w.ch.push('hello', helloData(nonce: 'nonce-9'));
    await w.idle();
    final ready = readyData(headSeq: head, gatewaySessionId: 'gs-2')
      ..['snapshot'] = snapshotJson(
        headSeq: head,
        unread: [
          if (snapshotUnread != null &&
              ((snapshotUnread['unread_count']! as int) > 0 ||
                  (snapshotUnread['mention_count']! as int) > 0))
            snapshotUnread,
        ],
      );
    w.ch.push('ready', ready);
    await w.idle();
    await w.dispatch([
      for (var seq = cursor + 1; seq <= head; seq++) history[seq]!,
    ]);
    await w.push('catchup.complete', {'head_seq': head});
    expect(
      w.catchUp.countsOf(t1),
      atHead,
      reason:
          'seed $seed: after the catch-up of ${cursor + 1}..$head '
          '(${[for (var seq = cursor + 1; seq <= head; seq++) history[seq]!['type']]})',
    );

    // Live tail after the head.
    await w.dispatch([
      for (var seq = head + 1; seq <= end; seq++) history[seq]!,
    ]);
    expect(
      w.catchUp.countsOf(t1),
      s.counts,
      reason: 'seed $seed: after the live tail ${head + 1}..$end',
    );
  } finally {
    await w.dispose();
  }
}

void main() {
  test(
    'a catch-up replay never changes a counter; live events after the head count '
    '(seeded model over created, updated, deleted and read state events)',
    () async {
      for (var seed = 1; seed <= 150; seed++) {
        await _run(seed);
      }
    },
    timeout: const Timeout(Duration(minutes: 5)),
  );
}
