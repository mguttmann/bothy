// SPDX-License-Identifier: Apache-2.0

/// Typed identifiers: one type per kind of object, each wrapping the canonical lowercase
/// UUID text, so a thread ID can never be passed where a message ID is expected.
library;

final RegExp _uuid = RegExp(
  r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$',
);

/// Checks [text] as a UUID in the 36-character form and returns it in lowercase.
///
/// [kind] names the identifier in the error. Throws a [FormatException] whose message names
/// only the length of the input, never the input itself, because an identifier can come from
/// an untrusted payload and end up in a log.
///
/// Example: `canonicalUuid('0192F4A0-7B1C-78F9-8074-FA94E84BAB7D', 'thread ID')` returns
/// `0192f4a0-7b1c-78f9-8074-fa94e84bab7d`.
String canonicalUuid(String text, String kind) {
  if (!_uuid.hasMatch(text)) {
    throw FormatException(
      'invalid $kind: expected a UUID of 36 characters, got ${text.length} characters',
    );
  }
  return text.toLowerCase();
}

/// ID of a space (03 `spaces.id`).
extension type const SpaceId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory SpaceId.parse(String text) =>
      SpaceId._(canonicalUuid(text, 'space ID'));
}

/// ID of a channel (03 `channels.id`).
extension type const ChannelId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory ChannelId.parse(String text) =>
      ChannelId._(canonicalUuid(text, 'channel ID'));
}

/// ID of a thread, fixed for its whole life, also across a move (03 `threads.id`).
extension type const ThreadId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory ThreadId.parse(String text) =>
      ThreadId._(canonicalUuid(text, 'thread ID'));
}

/// ID of a message (03 `messages.id`); also used for `client_msg_id` (04 5.1).
extension type const MessageId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory MessageId.parse(String text) =>
      MessageId._(canonicalUuid(text, 'message ID'));
}

/// ID of a run (03 `runs.id`).
extension type const RunId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory RunId.parse(String text) => RunId._(canonicalUuid(text, 'run ID'));
}

/// ID of a principal: person, agent or app (03 `principals.id`).
extension type const PrincipalId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory PrincipalId.parse(String text) =>
      PrincipalId._(canonicalUuid(text, 'principal ID'));
}

/// ID of a file (03 `files.id`).
extension type const FileId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory FileId.parse(String text) => FileId._(canonicalUuid(text, 'file ID'));
}

/// ID of a device (03 `devices.id`).
extension type const DeviceId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory DeviceId.parse(String text) =>
      DeviceId._(canonicalUuid(text, 'device ID'));
}

/// ID of a host of the terminal (24).
extension type const HostId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory HostId.parse(String text) => HostId._(canonicalUuid(text, 'host ID'));
}

/// ID of a terminal session (24).
extension type const TerminalSessionId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory TerminalSessionId.parse(String text) =>
      TerminalSessionId._(canonicalUuid(text, 'terminal session ID'));
}

/// ID of a message component (03 `message_components.id`).
extension type const ComponentId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory ComponentId.parse(String text) =>
      ComponentId._(canonicalUuid(text, 'component ID'));
}

/// ID of a slash command (03 `commands.id`).
extension type const CommandId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory CommandId.parse(String text) =>
      CommandId._(canonicalUuid(text, 'command ID'));
}

/// ID of a forum tag (03 `forum_tags.id`).
extension type const ForumTagId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory ForumTagId.parse(String text) =>
      ForumTagId._(canonicalUuid(text, 'forum tag ID'));
}

/// ID of a draft (03 `drafts.id`).
extension type const DraftId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory DraftId.parse(String text) =>
      DraftId._(canonicalUuid(text, 'draft ID'));
}

/// ID of a quiet hours rule (03 `quiet_hours.id`).
extension type const QuietHoursId._(String value) implements Object {
  /// Parses [text] in either case; throws a [FormatException] for anything but a UUID.
  factory QuietHoursId.parse(String text) =>
      QuietHoursId._(canonicalUuid(text, 'quiet hours ID'));
}
