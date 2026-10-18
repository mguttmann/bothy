// SPDX-License-Identifier: Apache-2.0

part of 'world.dart';

/// The writes and reads of the thread view (brief M4-20 Scope): capabilities, main threads,
/// operators, revisions, pins, templates, threads from a message, title, close, reopen, move
/// and mark unread. Each write appends the event of 05 section 6 like the other writes of the
/// world; the checks are the visibility, the level and the capability `manage` where 05 names
/// it, nothing else (README).
///
/// Example:
/// ```dart
/// world.grantCapability(alex.id, channel, 'manage');
/// world.pin(alex.id, messageId, pinned: true);
/// ```
extension ThreadViewWorld on FakeWorld {
  /// Gives [principalId] the capability [capability] (API `Capability`) in [channelId].
  void grantCapability(
    String principalId,
    String channelId,
    String capability,
  ) => channels[channelId]!.capabilities
      .putIfAbsent(principalId, () => {})
      .add(capability);

  /// Adds the main thread (`kind = main`) of the text channel [channelId], names it in the
  /// channel's `main_thread_id` and returns its ID.
  String addMainThread(String channelId, {required String createdBy}) {
    final id = addThread(channelId, 'main', createdBy: createdBy);
    threads[id]!['kind'] = 'main';
    channels[channelId]!.json['main_thread_id'] = id;
    return id;
  }

  /// Sets the effective operator of [channelId] to [agentId] (null: no operator).
  void setOperator(String channelId, String? agentId) =>
      channels[channelId]!.json['operator'] = {
        'mode': 'inherit',
        'effective_agent_id': ?agentId,
      };

  /// The effective operator of the thread [threadId]: its own operator, else its channel's.
  String? operatorOf(String threadId) {
    final thread = threads[threadId]!;
    final own = thread['operator_agent_id'] as String?;
    if (own != null) return own;
    final channel = channels[thread['channel_id']! as String]!;
    return (channel.json['operator'] as JsonMap?)?['effective_agent_id']
        as String?;
  }

  /// `listMessageRevisions`: the earlier revisions of [messageId], oldest first.
  List<JsonMap> revisionsOf(String principalId, String messageId) {
    visibleMessage(principalId, messageId);
    return [...?revisions[messageId]];
  }

  /// `markThreadUnread`: moves the read position of [principalId] in [threadId] back to
  /// [lastRead] when [expectedVersion] is the version of its read state (04 14), and appends
  /// `read_state.updated`. Throws `412 version_conflict` with the current state.
  JsonMap markUnread(
    String principalId,
    String threadId,
    int lastRead,
    int expectedVersion, {
    String? deviceId,
  }) {
    visibleThread(principalId, threadId);
    final current = readStates['$principalId|$threadId'];
    final version = (current?['version'] as int?) ?? 0;
    if (version != expectedVersion) {
      throw FakeProblem(412, 'version_conflict', current: current);
    }
    return putReadState(principalId, threadId, lastRead, deviceId: deviceId);
  }

  /// `pinMessage` and `unpinMessage`: pins or unpins [messageId] (05 `pin.create`,
  /// `pin.delete`: `manage`); a repeat changes nothing. Throws `404`, `403 forbidden` without
  /// `manage`, `410 message_deleted` for a deleted message.
  void pin(String principalId, String messageId, {required bool pinned}) {
    final m = visibleMessage(principalId, messageId);
    final threadId = m['thread_id']! as String;
    if (!manages(principalId, m['channel_id']! as String)) {
      throw const FakeProblem(403, 'forbidden');
    }
    if (m['deleted_at'] != null) {
      throw const FakeProblem(410, 'message_deleted');
    }
    if (m['pinned'] == pinned) return;
    m['pinned'] = pinned;
    final pin =
        pins[messageId] ??
        {
          'message_id': messageId,
          'thread_id': threadId,
          'pinned_by': principalId,
          'pinned_at': stamp(),
        };
    if (pinned) {
      pins[messageId] = pin;
    } else {
      pins.remove(messageId);
    }
    append(
      pinned ? 'pin.added' : 'pin.removed',
      resource: {'kind': 'message', 'id': messageId},
      scope: _scopeOf(threadId),
      payload: {'pin': pin},
      actor: principalId,
    );
  }

  /// `listPins`: the pins of [threadId].
  List<JsonMap> pinsOf(String principalId, String threadId) {
    visibleThread(principalId, threadId);
    return [
      for (final p in pins.values)
        if (p['thread_id'] == threadId) p,
    ];
  }

  /// `closeThread` and `reopenThread` of [principalId] (05: `write`); a thread already in
  /// the state changes nothing. Returns the thread.
  JsonMap setThreadState(
    String principalId,
    String threadId, {
    required bool closed,
  }) {
    final thread = _writable(principalId, threadId);
    if ((thread['state'] == 'closed') == closed) return thread;
    if (closed) {
      closeThread(threadId, actor: principalId);
    } else {
      _reopen(threadId, principalId);
    }
    return threads[threadId]!;
  }

  /// `setThreadTitle`: sets [title] with `title_source = manual` when [expectedVersion] is the
  /// thread's version, and appends `thread.updated`. Throws `412 version_conflict`.
  JsonMap setTitle(
    String principalId,
    String threadId,
    int expectedVersion,
    String title,
  ) {
    final thread = _writable(principalId, threadId);
    if (thread['version'] != expectedVersion) {
      throw FakeProblem(412, 'version_conflict', current: Map.of(thread));
    }
    thread
      ..['title'] = title
      ..['title_source'] = 'manual'
      ..['version'] = (thread['version']! as int) + 1;
    append(
      'thread.updated',
      resource: {
        'kind': 'thread',
        'id': threadId,
        'version': thread['version'],
      },
      scope: _scopeOf(threadId),
      payload: {'thread': thread},
      actor: principalId,
    );
    return thread;
  }

  /// `moveThread`: moves [threadId] into [targetChannelId] (05 `thread.move`: `manage` on
  /// source and target; the same effective operator, else `422 operator_mismatch`); the ID
  /// stays and `thread.moved` names the source channel.
  JsonMap moveThread(
    String principalId,
    String threadId,
    String targetChannelId,
  ) {
    final thread = visibleThread(principalId, threadId);
    final source = thread['channel_id']! as String;
    if (!seesChannel(principalId, targetChannelId)) {
      throw const FakeProblem(404, 'not_found');
    }
    if (!manages(principalId, source) ||
        !manages(principalId, targetChannelId)) {
      throw const FakeProblem(403, 'forbidden');
    }
    final operator = operatorOf(threadId);
    final target = channels[targetChannelId]!.json['operator'] as JsonMap?;
    if (operator != target?['effective_agent_id']) {
      throw const FakeProblem(422, 'operator_mismatch');
    }
    thread
      ..['channel_id'] = targetChannelId
      ..['moved_at'] = stamp()
      ..['version'] = (thread['version']! as int) + 1;
    for (final m in messagesOf(threadId)) {
      m['channel_id'] = targetChannelId;
    }
    append(
      'thread.moved',
      resource: {
        'kind': 'thread',
        'id': threadId,
        'version': thread['version'],
      },
      scope: _scopeOf(threadId),
      payload: {'thread': thread, 'from_channel_id': source},
      actor: principalId,
    );
    return thread;
  }

  /// `createThread` with `parent_message_id`: a thread from a message of the main thread of a
  /// text channel, titled from the message (`title_source = auto`), with an optional first
  /// message. Returns the API `ThreadCreated`.
  JsonMap createThreadFromMessage(
    String principalId,
    String channelId,
    JsonMap body,
  ) {
    final channel = channels[channelId];
    if (channel == null || !seesChannel(principalId, channelId)) {
      throw const FakeProblem(404, 'not_found');
    }
    if (channel.access[principalId] == 'read') {
      throw const FakeProblem(403, 'forbidden');
    }
    final parentId = body['parent_message_id']! as String;
    final parent = messages[parentId];
    if (parent == null ||
        parent['thread_id'] != channel.json['main_thread_id'] ||
        threads.values.any((t) => t['parent_message_id'] == parentId)) {
      throw const FakeProblem(
        400,
        'validation_failed',
        errors: [
          {'pointer': '/parent_message_id', 'code': 'invalid_parent'},
        ],
      );
    }
    final text = (parent['content_md']! as String).trim();
    final title = (body['title'] as String?)?.trim().isNotEmpty ?? false
        ? (body['title']! as String).trim()
        : text.isEmpty
        ? 'Thread'
        : text.split('\n').first;
    final id = addThread(channelId, title, createdBy: principalId);
    final thread = threads[id]!
      ..['parent_message_id'] = parentId
      ..['title_source'] = body['title'] == null ? 'auto' : 'manual';
    append(
      'thread.created',
      resource: {'kind': 'thread', 'id': id, 'version': thread['version']},
      scope: _scopeOf(id),
      payload: {'thread': thread},
      actor: principalId,
    );
    final first = body['first_message'] as JsonMap?;
    final message = first == null
        ? null
        : createMessage(principalId, id, first).message;
    return {'thread': thread, 'first_message': ?message};
  }

  /// Adds a template of [scope] (`personal` of [ownerId], or `channel` of [channelId]) and
  /// returns it.
  JsonMap addTemplate({
    required String scope,
    required String name,
    required String contentMd,
    String? ownerId,
    String? channelId,
  }) {
    final id = nextId();
    final template = <String, Object?>{
      'id': id,
      'scope': scope,
      'channel_id': ?channelId,
      'name': name,
      'content_md': contentMd,
      'version': 1,
    };
    templates[id] = template;
    if (ownerId != null) templateOwners[id] = ownerId;
    return template;
  }

  /// `listMyTemplates`: the personal templates of [principalId].
  List<JsonMap> personalTemplates(String principalId) => [
    for (final e in templates.entries)
      if (templateOwners[e.key] == principalId) e.value,
  ];

  /// `listTemplates`: the templates of [channelId] when [principalId] sees it.
  List<JsonMap> channelTemplates(String principalId, String channelId) {
    if (!seesChannel(principalId, channelId)) {
      throw const FakeProblem(404, 'not_found');
    }
    return [
      for (final t in templates.values)
        if (t['scope'] == 'channel' && t['channel_id'] == channelId) t,
    ];
  }

  JsonMap _writable(String principalId, String threadId) {
    final thread = visibleThread(principalId, threadId);
    if (levelOf(principalId, thread['channel_id']! as String) == 'read') {
      throw const FakeProblem(403, 'forbidden');
    }
    return thread;
  }
}
