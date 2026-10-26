// SPDX-License-Identifier: Apache-2.0

/// Instance information: branding, evaluated feature flags and limits (03 `instance_settings`; 15 6.1 `instance`; API `InstanceInfo`).
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';

/// Branding of the instance (API `Branding`); the app shows `productName`, never a literal.
final class Branding {
  /// Creates a [Branding].
  const Branding({
    required this.productName,
    required this.version,
    this.accentColor,
    this.logoFileId,
  });

  /// Product name.
  final String productName;

  /// Key of a category color.
  final String? accentColor;

  /// Logo file.
  final FileId? logoFileId;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Branding copyWith({
    String? productName,
    String? accentColor,
    FileId? logoFileId,
    ResourceVersion? version,
  }) => Branding(
    productName: productName ?? this.productName,
    accentColor: accentColor ?? this.accentColor,
    logoFileId: logoFileId ?? this.logoFileId,
    version: version ?? this.version,
  );

  @override
  bool operator ==(Object other) =>
      other is Branding &&
      other.productName == productName &&
      other.accentColor == accentColor &&
      other.logoFileId == logoFileId &&
      other.version == version;

  @override
  int get hashCode =>
      Object.hash(productName, accentColor, logoFileId, version);
}

/// Limits the server delivers (API `InstanceLimits`); never part of `AppConfig` (D-W-115).
final class InstanceLimits {
  /// Creates an [InstanceLimits].
  const InstanceLimits({
    required this.messageMaxContentBytes,
    required this.messageMaxAttachments,
    required this.fileMaxUploadBytes,
    required this.threadTitleMaxChars,
    required this.voiceMaxDurationMs,
  });

  /// Largest message content in bytes.
  final int messageMaxContentBytes;

  /// Most attachments per message.
  final int messageMaxAttachments;

  /// Largest upload in bytes.
  final int fileMaxUploadBytes;

  /// Longest thread title.
  final int threadTitleMaxChars;

  /// Longest voice message in milliseconds.
  final int voiceMaxDurationMs;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  InstanceLimits copyWith({
    int? messageMaxContentBytes,
    int? messageMaxAttachments,
    int? fileMaxUploadBytes,
    int? threadTitleMaxChars,
    int? voiceMaxDurationMs,
  }) => InstanceLimits(
    messageMaxContentBytes:
        messageMaxContentBytes ?? this.messageMaxContentBytes,
    messageMaxAttachments: messageMaxAttachments ?? this.messageMaxAttachments,
    fileMaxUploadBytes: fileMaxUploadBytes ?? this.fileMaxUploadBytes,
    threadTitleMaxChars: threadTitleMaxChars ?? this.threadTitleMaxChars,
    voiceMaxDurationMs: voiceMaxDurationMs ?? this.voiceMaxDurationMs,
  );

  @override
  bool operator ==(Object other) =>
      other is InstanceLimits &&
      other.messageMaxContentBytes == messageMaxContentBytes &&
      other.messageMaxAttachments == messageMaxAttachments &&
      other.fileMaxUploadBytes == fileMaxUploadBytes &&
      other.threadTitleMaxChars == threadTitleMaxChars &&
      other.voiceMaxDurationMs == voiceMaxDurationMs;

  @override
  int get hashCode => Object.hash(
    messageMaxContentBytes,
    messageMaxAttachments,
    fileMaxUploadBytes,
    threadTitleMaxChars,
    voiceMaxDurationMs,
  );
}

/// The app lock policy of the instance (API `AppLockPolicy`; docs/spec/08-identity.md 9).
///
/// Both values are process configuration of the server: [required] is
/// `security.app_lock_required`, [afterMs] is `client.app_lock_after` in milliseconds. The
/// server cannot check the lock; the phone app enforces it (brief M5-08 Design note 1).
final class AppLockPolicy {
  /// Creates an [AppLockPolicy].
  const AppLockPolicy({required this.required, required this.afterMs});

  /// Whether the owner requires the app lock on every device.
  final bool required;

  /// Time without use after which the app locks, in milliseconds.
  final int afterMs;

  /// [afterMs] as a [Duration].
  Duration get after => Duration(milliseconds: afterMs);

  @override
  bool operator ==(Object other) =>
      other is AppLockPolicy &&
      other.required == required &&
      other.afterMs == afterMs;

  @override
  int get hashCode => Object.hash(required, afterMs);
}

/// Information about the instance (API `InstanceInfo`).
final class InstanceInfo {
  /// Creates an [InstanceInfo].
  const InstanceInfo({
    required this.instanceId,
    required this.branding,
    required this.localeDefault,
    required this.specVersion,
    required this.features,
    required this.limits,
    this.appLock,
  });

  /// ID of the instance.
  final String instanceId;

  /// Branding.
  final Branding branding;

  /// Default locale.
  final String localeDefault;

  /// Version of the specification the server implements.
  final String specVersion;

  /// Evaluated feature flags by key.
  final Map<String, bool> features;

  /// Limits.
  final InstanceLimits limits;

  /// The app lock policy; null in an instance row stored before the policy existed, until the
  /// next `getInstance` replaces it.
  final AppLockPolicy? appLock;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  InstanceInfo copyWith({
    String? instanceId,
    Branding? branding,
    String? localeDefault,
    String? specVersion,
    Map<String, bool>? features,
    InstanceLimits? limits,
    AppLockPolicy? appLock,
  }) => InstanceInfo(
    instanceId: instanceId ?? this.instanceId,
    branding: branding ?? this.branding,
    localeDefault: localeDefault ?? this.localeDefault,
    specVersion: specVersion ?? this.specVersion,
    features: features ?? this.features,
    limits: limits ?? this.limits,
    appLock: appLock ?? this.appLock,
  );

  @override
  bool operator ==(Object other) =>
      other is InstanceInfo &&
      other.instanceId == instanceId &&
      other.branding == branding &&
      other.localeDefault == localeDefault &&
      other.specVersion == specVersion &&
      jsonEquals(other.features, features) &&
      other.limits == limits &&
      other.appLock == appLock;

  @override
  int get hashCode => Object.hash(
    instanceId,
    branding,
    localeDefault,
    specVersion,
    jsonHash(features),
    limits,
    appLock,
  );
}
