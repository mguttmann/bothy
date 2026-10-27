// SPDX-License-Identifier: Apache-2.0

/// The version gate of the apps (docs/spec/18-release.md 9 item 2, R-RL-08; brief M7-13 Design
/// note 1): semantic versions and the decision whether an app version may connect, given the
/// per-platform rules `client.min_version.<platform>` and `client.recommended_version.<platform>`
/// that `getDiscovery` returns in `min_client_versions`.
library;

/// A semantic version of SemVer 2.0.0 (https://semver.org/spec/v2.0.0.html).
///
/// Ordered by the precedence of item 11: `major.minor.patch` numerically, a pre-release below
/// its release, pre-release identifiers one by one (numeric ones numerically and below
/// alphanumeric ones, a shorter list below a longer one with the same start); build metadata
/// does not count.
///
/// Example: `SemVer.parse('1.0.0-beta.2') < SemVer.parse('1.0.0-beta.11')` is true.
final class SemVer implements Comparable<SemVer> {
  const SemVer._(
    this.major,
    this.minor,
    this.patch,
    this.preRelease,
    this.build,
  );

  /// Parses [text], or returns null when it is no SemVer 2.0.0 version (no `v` prefix, no
  /// spaces, no leading zeros in numbers).
  static SemVer? tryParse(String text) {
    final m = _pattern.firstMatch(text);
    if (m == null) return null;
    final pre = m.group(4);
    final build = m.group(5);
    return SemVer._(
      int.parse(m.group(1)!),
      int.parse(m.group(2)!),
      int.parse(m.group(3)!),
      pre == null ? const [] : List.unmodifiable(pre.split('.')),
      build == null ? const [] : List.unmodifiable(build.split('.')),
    );
  }

  /// Parses [text].
  ///
  /// Throws a [FormatException] when [text] is no SemVer 2.0.0 version.
  static SemVer parse(String text) =>
      tryParse(text) ?? (throw FormatException('not a semantic version', text));

  // The regular expression of semver.org, item "Is there a suggested regular expression".
  static final RegExp _pattern = RegExp(
    r'^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)'
    r'(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)'
    r'(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?'
    r'(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$',
  );

  /// The major version.
  final int major;

  /// The minor version.
  final int minor;

  /// The patch version.
  final int patch;

  /// The pre-release identifiers, empty for a release.
  final List<String> preRelease;

  /// The build metadata identifiers; they do not take part in the order.
  final List<String> build;

  @override
  int compareTo(SemVer other) {
    for (final d in [
      major - other.major,
      minor - other.minor,
      patch - other.patch,
    ]) {
      if (d != 0) return d;
    }
    // A release ranks above every pre-release of the same version.
    if (preRelease.isEmpty || other.preRelease.isEmpty) {
      return (preRelease.isEmpty ? 1 : 0) - (other.preRelease.isEmpty ? 1 : 0);
    }
    final n = preRelease.length < other.preRelease.length
        ? preRelease.length
        : other.preRelease.length;
    for (var i = 0; i < n; i++) {
      final d = _compareIdentifier(preRelease[i], other.preRelease[i]);
      if (d != 0) return d;
    }
    return preRelease.length - other.preRelease.length;
  }

  static int _compareIdentifier(String a, String b) {
    final x = int.tryParse(a);
    final y = int.tryParse(b);
    if (x != null && y != null) return x.compareTo(y);
    if (x != null) return -1;
    if (y != null) return 1;
    return a.compareTo(b);
  }

  /// Whether this version ranks below [other].
  bool operator <(SemVer other) => compareTo(other) < 0;

  @override
  bool operator ==(Object other) => other is SemVer && compareTo(other) == 0;

  @override
  int get hashCode =>
      Object.hash(major, minor, patch, Object.hashAll(preRelease));

  @override
  String toString() =>
      '$major.$minor.$patch'
      '${preRelease.isEmpty ? '' : '-${preRelease.join('.')}'}'
      '${build.isEmpty ? '' : '+${build.join('.')}'}';
}

/// One entry of `min_client_versions` of `getDiscovery` (API `ClientVersionRequirement`).
final class ClientVersionRule {
  /// Creates the rule.
  const ClientVersionRule({
    required this.platform,
    required this.minVersion,
    this.recommendedVersion,
  });

  /// The `DevicePlatform` the rule applies to (`ios`, `android`, `macos`, `windows`).
  final String platform;

  /// `client.min_version.<platform>`: below it the app does not connect.
  final String minVersion;

  /// `client.recommended_version.<platform>`: below it the app shows a notice; null for none.
  final String? recommendedVersion;
}

/// What the version gate decided for this app.
sealed class ClientVersionDecision {
  const ClientVersionDecision();
}

/// No rule holds the app back.
final class ClientVersionAllowed extends ClientVersionDecision {
  /// Creates the decision.
  const ClientVersionAllowed();

  @override
  bool operator ==(Object other) => other is ClientVersionAllowed;

  @override
  int get hashCode => (ClientVersionAllowed).hashCode;

  @override
  String toString() => 'ClientVersionAllowed()';
}

/// The app is below `client.recommended_version.<platform>`: it works and shows a notice.
final class ClientVersionUpdateRecommended extends ClientVersionDecision {
  /// Creates the decision.
  const ClientVersionUpdateRecommended(this.recommendedVersion);

  /// The version the instance recommends.
  final String recommendedVersion;

  @override
  bool operator ==(Object other) =>
      other is ClientVersionUpdateRecommended &&
      other.recommendedVersion == recommendedVersion;

  @override
  int get hashCode => recommendedVersion.hashCode;

  @override
  String toString() => 'ClientVersionUpdateRecommended($recommendedVersion)';
}

/// The app is below `client.min_version.<platform>`: it does not connect and shows the update
/// path of its channel.
final class ClientVersionBelowMinimum extends ClientVersionDecision {
  /// Creates the decision.
  const ClientVersionBelowMinimum(this.minVersion);

  /// The version the instance requires.
  final String minVersion;

  @override
  bool operator ==(Object other) =>
      other is ClientVersionBelowMinimum && other.minVersion == minVersion;

  @override
  int get hashCode => minVersion.hashCode;

  @override
  String toString() => 'ClientVersionBelowMinimum($minVersion)';
}

/// Decides the version gate for an app of [platform] in [appVersion] under [rules] (18 9
/// item 2).
///
/// Only the first rule of [platform] applies. Versions compare as SemVer 2.0.0, so a
/// pre-release of the minimum is below it. A rule value that is no SemVer counts as absent and
/// is reported to [onUnparsable] with its configuration key; an [appVersion] that is no SemVer
/// is reported as `app version` and never blocked, because the server stays the second line
/// with `4007` (04 6.4). The minimum wins over the recommendation.
///
/// Example: `decideClientVersion(platform: 'ios', appVersion: '1.0.0', rules: [ClientVersionRule(platform: 'ios', minVersion: '1.1.0')])`
/// is `ClientVersionBelowMinimum('1.1.0')`.
ClientVersionDecision decideClientVersion({
  required String platform,
  required String appVersion,
  required Iterable<ClientVersionRule> rules,
  void Function(String key, String value)? onUnparsable,
}) {
  ClientVersionRule? rule;
  for (final r in rules) {
    if (r.platform == platform) {
      rule = r;
      break;
    }
  }
  if (rule == null) return const ClientVersionAllowed();
  SemVer? read(String key, String? value) {
    if (value == null) return null;
    final v = SemVer.tryParse(value);
    if (v == null) onUnparsable?.call('$key.$platform', value);
    return v;
  }

  final min = read('client.min_version', rule.minVersion);
  final recommended = read(
    'client.recommended_version',
    rule.recommendedVersion,
  );
  if (min == null && recommended == null) return const ClientVersionAllowed();
  final app = SemVer.tryParse(appVersion);
  if (app == null) {
    onUnparsable?.call('app version', appVersion);
    return const ClientVersionAllowed();
  }
  if (min != null && app < min) {
    return ClientVersionBelowMinimum(rule.minVersion);
  }
  if (recommended != null && app < recommended) {
    return ClientVersionUpdateRecommended(rule.recommendedVersion!);
  }
  return const ClientVersionAllowed();
}
