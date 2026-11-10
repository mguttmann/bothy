// SPDX-License-Identifier: Apache-2.0

part of 'world.dart';

/// Wake-ups of stage AR as the server of AR-04 keeps them (09 5.8, 07 2.14; brief AR-08): a
/// pending wake-up of an agent's run, its end with the note of 09 5.8.4 (a message with origin
/// `system` and kind `text`, authored by the agent, whose English text names the label and the
/// reason, D-AR-04-6), a wake run started by a fired wake-up (`trigger_kind = wake`), the
/// chain and idle cap note of 09 5.8.6, and the channel switch `allow_autonomous_runs`. Every
/// change appends `run.wake_changed` (payload of D-AR-04-5). The fake has no job
/// `runs.wake_due` and no checks of 5.8.3: a test fires and ends wake-ups itself.
///
/// Example:
/// ```dart
/// final wake = world.addWake(threadId: thread, agentId: robin, sourceRunId: run,
///     triggeredBy: alex, label: 'CI of #123');
/// world.endWake(wake['id']! as String, 'expired', reason: 'deadline_passed');
/// ```
extension WakesWorld on FakeWorld {
  /// Adds a pending wake-up of [agentId] in [threadId] for the person [triggeredBy] of the run
  /// [sourceRunId] and appends `run.wake_changed`; returns its API `RunWake`. [notBefore] and
  /// [deadline] default to 20 minutes and 7 days from now.
  JsonMap addWake({
    required String threadId,
    required String agentId,
    required String sourceRunId,
    required String triggeredBy,
    String kind = 'at',
    String reason = 'agent',
    String label = '',
    JsonMap condition = const {},
    DateTime? notBefore,
    DateTime? deadline,
    int? outage,
  }) {
    final id = nextId();
    final at = now();
    final wake = <String, Object?>{
      'id': id,
      'agent_id': agentId,
      'thread_id': threadId,
      'source_run_id': sourceRunId,
      'triggered_by': triggeredBy,
      'kind': kind,
      'reason': reason,
      'label': label,
      'condition': {...condition},
      'outage': ?outage,
      'not_before': stamp(notBefore ?? at.add(const Duration(minutes: 20))),
      'deadline': stamp(deadline ?? at.add(const Duration(days: 7))),
      'state': 'pending',
      'result_text': '',
      'created_at': stamp(at),
      'version': 1,
    };
    wakes[id] = wake;
    _wakeChanged(wake);
    return wake;
  }

  /// The wake-ups of [threadId], newest first (`listThreadWakes`, 09 5.8.2).
  List<JsonMap> wakesOf(String threadId) => [
    for (final w in wakes.values.toList().reversed)
      if (w['thread_id'] == threadId) w,
  ];

  /// Ends the pending wake-up [wakeId] with [state] (`cancelled`, `expired`, `refused` or
  /// `failed`) and [reason], posts the note of 09 5.8.4 into its thread and appends
  /// `run.wake_changed` naming the note; an ended wake-up stays as it is.
  JsonMap endWake(String wakeId, String state, {String? reason}) {
    final wake = wakes[wakeId]!;
    if (wake['state'] != 'pending') return wake;
    wake
      ..['state'] = state
      ..['ended_at'] = stamp()
      ..['version'] = (wake['version']! as int) + 1;
    if (reason != null) wake['state_reason'] = reason;
    final note = _systemNote(
      wake['thread_id']! as String,
      wakeNoteText(wake['label']! as String, state, reason ?? ''),
      wake['agent_id']! as String,
    );
    _wakeChanged(wake, note: note);
    return wake;
  }

  /// Fires the pending wake-up [wakeId]: a wake run of its agent for its person in its thread
  /// (`trigger_kind = wake`, `triggering_wake_id`, 09 5.8.3) with `run.state_changed`, the
  /// wake-up `fired` with `fired_run_id` and `run.wake_changed`; returns the run document.
  JsonMap fireWake(String wakeId) {
    final wake = wakes[wakeId]!;
    final threadId = wake['thread_id']! as String;
    final runId = nextId();
    final run = <String, Object?>{
      'id': runId,
      'thread_id': threadId,
      'channel_id': threads[threadId]!['channel_id'],
      'agent_id': wake['agent_id'],
      'trigger_kind': 'wake',
      'triggering_wake_id': wakeId,
      'triggered_by': wake['triggered_by'],
      'state': 'queued',
      'version': 1,
      'billing_mode_expected': 'organization',
      'metrics': <String, Object?>{},
      'cost': {'amount_micros': 0, 'currency': 'EUR', 'is_estimate': true},
      'steps': <String, Object?>{},
      'followups': <String, Object?>{},
      'queued_at': stamp(),
    };
    runs[runId] = run;
    _runStateEvent(run);
    wake
      ..['state'] = 'fired'
      ..['state_reason'] = 'condition_met'
      ..['fired_run_id'] = runId
      ..['ended_at'] = stamp()
      ..['version'] = (wake['version']! as int) + 1;
    _wakeChanged(wake);
    return run;
  }

  /// Posts the note of a chain that reached a cap of 09 5.8.6 into [threadId] as [agentId]
  /// (`autonomous_chain_max` or `autonomous_idle_max`); returns the message.
  JsonMap autonomyNote(String threadId, String agentId, String reason) =>
      _systemNote(
        threadId,
        'Autonomous work stopped ($reason): a reply continues it',
        agentId,
      );

  /// Switches `allow_autonomous_runs` of [channelId] (`setChannelAutonomousRuns`): the version
  /// rises and `channel.updated` follows; the same value changes nothing.
  JsonMap setAutonomousRuns(String channelId, {required bool allowed}) {
    final channel = channels[channelId]!;
    if (channel.json['allow_autonomous_runs'] == allowed) return channel.json;
    final json = channel.json
      ..['allow_autonomous_runs'] = allowed
      ..['version'] = (channel.json['version']! as int) + 1;
    append(
      'channel.updated',
      resource: {
        'kind': 'channel',
        'id': channelId,
        'version': json['version'],
      },
      scope: {'space_id': ?channel.spaceId, 'channel_id': channelId},
      payload: {'channel': json},
    );
    return json;
  }

  /// Makes every channel look like one of a server before stage AR: without
  /// `allow_autonomous_runs` (tests of the hidden switch).
  void withoutAutonomousRunsSwitch() {
    for (final c in channels.values) {
      c.json.remove('allow_autonomous_runs');
    }
  }

  /// The English note of 09 5.8.4 as the server writes it (`domain.WakeNoteText` of AR-04).
  static String wakeNoteText(String label, String state, String reason) {
    final name = label.isEmpty ? 'Wake-up' : "Wake-up '$label'";
    switch (state) {
      case 'expired':
        return '$name expired: its deadline passed before it could fire';
      case 'cancelled':
        return '$name cancelled ($reason)';
      case 'failed':
        return '$name failed: its check could not run ($reason)';
      case 'refused':
        if (reason == 'autonomous_chain_max' ||
            reason == 'autonomous_idle_max') {
          return '$name refused ($reason): a reply of the person continues the work';
        }
        if (reason == 'autonomous_not_allowed') {
          return '$name refused: agents may not run on alone in this channel';
        }
        return '$name refused ($reason)';
    }
    return '$name ended ($reason)';
  }

  void _wakeChanged(JsonMap wake, {JsonMap? note}) {
    final threadId = wake['thread_id']! as String;
    append(
      'run.wake_changed',
      resource: {'kind': 'thread', 'id': threadId},
      scope: _scopeOf(threadId),
      payload: {
        'wake_id': wake['id'],
        'thread_id': threadId,
        'agent_id': wake['agent_id'],
        'triggered_by': wake['triggered_by'],
        'state': wake['state'],
        'state_reason': ?wake['state_reason'],
        'note_message_id': ?note?['id'],
        'note_thread_seq': ?note?['thread_seq'],
      },
    );
  }

  /// Writes a note (`origin = system`, `kind = text`, silent) with [text] into [threadId] as
  /// [authorId] and appends `message.created`, as `messaging.AppendSystemNote` does.
  JsonMap _systemNote(String threadId, String text, String authorId) =>
      _systemMessage(threadId, authorId, kind: 'text', text: text);
}
