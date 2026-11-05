// SPDX-License-Identifier: Apache-2.0

part of 'world.dart';

/// Creating spaces and channels with their live events, as the server does
/// (`createSpace`, `createChannel`; 05 section 6): `space.created` with the space and
/// `channel.created` with the channel and nothing else. Like the server, the payload carries no
/// level and no capabilities of anyone, and the creator (an owner, who holds every capability
/// through the owner rule, 07) gets no `access.changed`; a client learns its access only from
/// the `SpaceView` and `ChannelView` of the snapshot or REST. The fake has no permission logic
/// beyond the levels of [FakeChannel.access] and its capabilities (README): a space is visible
/// through its channels, so [createSpace] creates its first channel with it.
///
/// Example:
/// ```dart
/// final forum = world.createChannel(alex.id, spaceId, 'website',
///     {alex.id: 'trigger'}, kind: 'forum', projectBound: true,
///     capabilities: {alex.id: {'manage'}});
/// ```
extension StructureEventsWorld on FakeWorld {
  /// Creates the space [name] by [actor] with its first text channel [channelName], which the
  /// principals of [access] see with their level and [capabilities]; appends `space.created`
  /// and `channel.created` and returns both IDs.
  ({String space, String channel}) createSpace(
    String actor,
    String name, {
    required Map<String, String> access,
    String channelName = 'general',
    Map<String, Set<String>> capabilities = const {},
  }) {
    final space = addSpace(name);
    // The channel exists before the events, so the gateway hands `space.created` to the
    // principals who see the space through it.
    final channel = _addChannel(space, channelName, access, capabilities);
    final json = spaces[space]!;
    append(
      'space.created',
      resource: {'kind': 'space', 'id': space, 'version': json['version']},
      scope: {'space_id': space},
      payload: {'space': json},
      actor: actor,
    );
    _channelCreated(actor, channel);
    return (space: space, channel: channel);
  }

  /// Creates the channel [name] of [kind] (`text` or `forum`) in [spaceId] by [actor], seen by
  /// the principals of [access] with their level and holding [capabilities] (for example
  /// `manage` of an owner); [projectBound] marks a forum with a project binding. Appends
  /// `channel.created` with only the channel and returns its ID.
  String createChannel(
    String actor,
    String spaceId,
    String name,
    Map<String, String> access, {
    String kind = 'text',
    bool projectBound = false,
    Map<String, Set<String>> capabilities = const {},
  }) {
    final id = _addChannel(
      spaceId,
      name,
      access,
      capabilities,
      kind: kind,
      projectBound: projectBound,
    );
    _channelCreated(actor, id);
    return id;
  }

  String _addChannel(
    String spaceId,
    String name,
    Map<String, String> access,
    Map<String, Set<String>> capabilities, {
    String kind = 'text',
    bool projectBound = false,
  }) {
    final id = kind == 'forum'
        ? addForum(spaceId, name, access)
        : addChannel(spaceId, name, access);
    final channel = channels[id]!;
    channel.json['project_bound'] = projectBound;
    for (final e in capabilities.entries) {
      channel.capabilities[e.key] = {...e.value};
    }
    return id;
  }

  void _channelCreated(String actor, String id) {
    final channel = channels[id]!;
    append(
      'channel.created',
      resource: {
        'kind': 'channel',
        'id': id,
        'version': channel.json['version'],
      },
      scope: {'space_id': ?channel.spaceId, 'channel_id': id},
      payload: {'channel': channel.json},
      actor: actor,
    );
  }
}
