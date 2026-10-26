// SPDX-License-Identifier: Apache-2.0

/// The Pigeon source of the sharing channel between the app and the Swift glue of its iOS and
/// macOS runners (brief M5-06 Design notes 3 to 5 and 7; docs/decisions.d/M5-06.md).
///
/// Regenerate with `dart run pigeon --input pigeons/share_apple.dart` in `apps/client`;
/// `make codegen-check` regenerates and compares. The generated files are
/// `lib/src/generated/share_apple_api.g.dart` and `ios/Runner/ShareAppleApi.g.swift`; the macOS
/// runner compiles the same Swift file.
library;

import 'package:pigeon/pigeon.dart';

/// The child session of the Share Extension as `createExtensionSession` returned it, for the
/// shared keychain (08 7.4). Only the runner keeps the tokens; Dart never persists them.
class ExtensionSessionRecord {
  /// Creates the record.
  ExtensionSessionRecord({
    required this.instanceId,
    required this.serverUrl,
    required this.sessionId,
    required this.accessToken,
    required this.accessExpiresAtMillis,
    required this.refreshToken,
    required this.refreshExpiresAtMillis,
  });

  /// The `instance_id`, the scope of the device keys and of the keychain item.
  String instanceId;

  /// The server address (origin and base path) of the instance.
  String serverUrl;

  /// The ID of the child session.
  String sessionId;

  /// The access token of the child session.
  String accessToken;

  /// When the access token expires, in milliseconds since the epoch (UTC).
  int accessExpiresAtMillis;

  /// The refresh token of the child's own family, shown once by the server.
  String refreshToken;

  /// When the refresh token expires, in milliseconds since the epoch (UTC).
  int refreshExpiresAtMillis;
}

/// The values the Share Extension reads from the App Group besides the targets: the instance
/// limits of `getInstance` (D-W-115) and `client.share.recent_targets` of `AppConfig`.
class ShareSettingsRecord {
  /// Creates the record.
  ShareSettingsRecord({
    required this.instanceId,
    required this.recentTargetsMax,
    this.fileMaxUploadBytes,
    this.messageMaxAttachments,
  });

  /// The `instance_id` of the signed-in account.
  String instanceId;

  /// `client.share.recent_targets` of `AppConfig`.
  int recentTargetsMax;

  /// `file_max_upload_bytes` of `InstanceLimits`, or null while unknown (the server checks).
  int? fileMaxUploadBytes;

  /// `message_max_attachments` of `InstanceLimits`, or null while unknown (the server checks).
  int? messageMaxAttachments;
}

/// One target of the share sheet (brief M5-06 Design notes 1 and 7).
class ShareTargetRecord {
  /// Creates the target.
  ShareTargetRecord({
    required this.kind,
    required this.channelId,
    required this.path,
    this.threadId,
  });

  /// `thread` (a thread or forum post), `channel` (the main thread of a text channel) or
  /// `new_post` (a new post in a forum channel).
  String kind;

  /// The channel of the target; uploads go to `/channels/{channel_id}/uploads`.
  String channelId;

  /// The thread messages go to: the thread itself, or the channel's `main_thread_id`; null for
  /// `new_post`.
  String? threadId;

  /// The display path, channel and thread names the person can see, for example
  /// `server > Gateway: resume`.
  String path;
}

/// One file of an inbox entry.
class ShareInboxFileRecord {
  /// Creates the file.
  ShareInboxFileRecord({
    required this.path,
    required this.name,
    required this.mimeType,
    required this.sizeBytes,
    this.completedFileId,
  });

  /// The absolute path of the file in the App Group container.
  String path;

  /// The original file name.
  String name;

  /// The declared MIME type.
  String mimeType;

  /// The size in bytes.
  int sizeBytes;

  /// The file ID when the extension completed its upload already, else null.
  String? completedFileId;
}

/// One entry of the share inbox of the App Group: a share the extension could not send
/// (brief M5-06 Design note 4).
class ShareInboxEntryRecord {
  /// Creates the entry.
  ShareInboxEntryRecord({
    required this.clientMsgId,
    required this.instanceId,
    required this.target,
    required this.comment,
    required this.createdAtMillis,
    required this.files,
    this.title,
  });

  /// The `client_msg_id` the extension generated; the app sends with the same ID.
  String clientMsgId;

  /// The instance the share was meant for.
  String instanceId;

  /// The chosen target.
  ShareTargetRecord target;

  /// The text of the message: the person's comment and shared text or links.
  String comment;

  /// The title of a new forum post (`new_post`), else null.
  String? title;

  /// When the extension wrote the entry, in milliseconds since the epoch (UTC).
  int createdAtMillis;

  /// The shared files in their order.
  List<ShareInboxFileRecord> files;
}

/// The sharing glue of the iOS and macOS runners over ClientKit: the child session in the
/// shared keychain, the settings and recent targets in the App Group, and the share inbox.
///
/// Every method that needs the App Group or the shared keychain throws a `PlatformException`
/// with the code `unavailable` in a build without them (a build without a signing team,
/// D-M5-04-1); [isAvailable] says so up front.
@ConfigurePigeon(
  PigeonOptions(
    dartOut: 'lib/src/generated/share_apple_api.g.dart',
    dartPackageName: 'client',
    swiftOut: 'ios/Runner/ShareAppleApi.g.swift',
    swiftOptions: SwiftOptions(errorClassName: 'ShareApplePigeonError'),
    copyrightHeader: 'pigeons/copyright_share_apple.txt',
  ),
)
@HostApi()
abstract class ShareAppleHostApi {
  /// Whether this build has the App Group and the shared keychain group the Share Extension
  /// needs.
  bool isAvailable();

  /// Stores the child session [record] in the shared keychain, replacing the earlier one of
  /// its instance.
  void saveExtensionSession(ExtensionSessionRecord record);

  /// Whether the shared keychain holds a child session of [instanceId] whose refresh token is
  /// still valid at [nowMillis]; the extension deletes a revoked one.
  bool hasExtensionSession(String instanceId, int nowMillis);

  /// Deletes the child session of [instanceId] from the shared keychain.
  void deleteExtensionSession(String instanceId);

  /// Writes the settings the extension reads.
  void writeShareSettings(ShareSettingsRecord settings);

  /// Replaces the recent targets of the App Group with [targets], newest first.
  void writeRecentTargets(List<ShareTargetRecord> targets);

  /// Removes the recent targets, the settings, the share inbox and the staging folders from
  /// the App Group (logout and every other end of the account).
  void clearShareData();

  /// Every complete entry of the share inbox, oldest first.
  List<ShareInboxEntryRecord> readInbox();

  /// Deletes the inbox entry [clientMsgId] with its files.
  void deleteInboxEntry(String clientMsgId);
}
