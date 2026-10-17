// SPDX-License-Identifier: Apache-2.0

/// Display names for the IDs of the statistics types.
library;

/// Names of agents, spaces and channels by ID, supplied by the app or the harness.
///
/// The statistics types carry only IDs (docs/spec/25-visualization.md 5.2); the names come from
/// the local data of the app. An ID without a name shows its last [_shortLength] characters, so a
/// card stays readable before the directory has loaded.
///
/// ```dart
/// const names = VizNames(agents: {'0192...0202': 'dev-runner'});
/// names.agent('0192...0202'); // 'dev-runner'
/// ```
final class VizNames {
  /// Creates the lookup; every map defaults to empty.
  const VizNames({
    this.agents = const {},
    this.spaces = const {},
    this.channels = const {},
  });

  /// Agent name by agent ID.
  final Map<String, String> agents;

  /// Space name by space ID.
  final Map<String, String> spaces;

  /// Channel name by channel ID, for example `#backend`.
  final Map<String, String> channels;

  static const int _shortLength = 6;

  /// The name of agent [id], or its short form.
  String agent(String id) => agents[id] ?? short(id);

  /// The name of space [id], or its short form.
  String space(String id) => spaces[id] ?? short(id);

  /// The name of channel [id], or its short form.
  String channel(String id) => channels[id] ?? short(id);

  /// The name of [id] as an agent, a space or a channel, or its short form: the group key of a
  /// query response, which can name any of them.
  String any(String id) =>
      agents[id] ?? spaces[id] ?? channels[id] ?? short(id);

  /// The last characters of [id]: the random part of a UUIDv7, which tells IDs apart.
  static String short(String id) =>
      id.length <= _shortLength ? id : id.substring(id.length - _shortLength);
}
