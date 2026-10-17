// SPDX-License-Identifier: Apache-2.0

/// Ordered counters of the sync protocol, each its own type so they cannot be mixed
/// (04 3: global `seq`; 04 4: `thread_seq`; 04 9: `resource.version`; 04 2.2:
/// `permission_version`).
library;

/// Global, commit-ordered sequence of a durable event (04 3); also the client's cursor (04 7.1).
extension type const GlobalSeq(int value) {
  /// Whether this sequence is lower than [other].
  bool operator <(GlobalSeq other) => value < other.value;

  /// Whether this sequence is lower than or equal to [other].
  bool operator <=(GlobalSeq other) => value <= other.value;

  /// Whether this sequence is greater than [other].
  bool operator >(GlobalSeq other) => value > other.value;

  /// Whether this sequence is greater than or equal to [other].
  bool operator >=(GlobalSeq other) => value >= other.value;
}

/// Gapless position of a message in its thread (04 4); `0` means "before the first message".
extension type const ThreadSeq(int value) {
  /// Whether this position is lower than [other].
  bool operator <(ThreadSeq other) => value < other.value;

  /// Whether this position is lower than or equal to [other].
  bool operator <=(ThreadSeq other) => value <= other.value;

  /// Whether this position is greater than [other].
  bool operator >(ThreadSeq other) => value > other.value;

  /// Whether this position is greater than or equal to [other].
  bool operator >=(ThreadSeq other) => value >= other.value;

  /// The position directly after this one.
  ThreadSeq get next => ThreadSeq(value + 1);
}

/// Version of an object after a change (03 `version` columns, 04 9 item 3).
extension type const ResourceVersion(int value) {
  /// Whether this version is lower than [other].
  bool operator <(ResourceVersion other) => value < other.value;

  /// Whether this version is lower than or equal to [other].
  bool operator <=(ResourceVersion other) => value <= other.value;

  /// Whether this version is greater than [other].
  bool operator >(ResourceVersion other) => value > other.value;

  /// Whether this version is greater than or equal to [other].
  bool operator >=(ResourceVersion other) => value >= other.value;
}

/// Permission version at the time of a commit (04 2.2 `permission_version`).
extension type const PermissionVersion(int value) {
  /// Whether this version is lower than [other].
  bool operator <(PermissionVersion other) => value < other.value;

  /// Whether this version is greater than [other].
  bool operator >(PermissionVersion other) => value > other.value;
}
