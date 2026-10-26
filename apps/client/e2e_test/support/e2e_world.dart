// SPDX-License-Identifier: Apache-2.0

// The world of a desktop E2E run (brief M4-34 Design note 1): what `bothy-smoke e2e-world` wrote
// into e2e.json, plus the test CA of the TLS front. `tool/e2e_suite.sh` hands both to the test
// build as `--dart-define=E2E_CONFIG=<base64 of e2e.json>` and `--dart-define=E2E_CA=<base64 of
// the PEM>`: a desktop test app runs in its own sandbox and cannot read the files of the stack,
// and a define is the one input `flutter test` and `flutter drive` pass to every device. The
// build of a run is local and the stack with its codes is removed after it.

import 'dart:convert';

/// One person of the world: the ID, the handle and the last enrollment code issued for them
/// (the only valid one, 08 3.2); for Alex also the password and TOTP secret of the local login.
final class E2ePerson {
  E2ePerson._(Map<String, Object?> json)
    : id = json['id']! as String,
      handle = json['handle']! as String,
      displayName = json['display_name']! as String,
      code = json['code']! as String,
      password = json['password'] as String?,
      totpSecret = json['totp_secret'] as String?,
      lastStep = (json['last_step'] as num?)?.toInt() ?? 0;

  /// The principal ID.
  final String id;

  /// The handle, the login name of the local account.
  final String handle;

  /// The display name.
  final String displayName;

  /// The enrollment code e2e.json carries; the first actor of the person redeems it.
  final String code;

  /// The password of the local account, or null.
  final String? password;

  /// The TOTP secret (base32 without padding), or null.
  final String? totpSecret;

  /// The TOTP step the world already used; a login must use a later one (08 2.5).
  final int lastStep;
}

/// An agent of the world: its ID and the handle a mention uses.
final class E2eAgentRef {
  E2eAgentRef._(Map<String, Object?> json)
    : id = json['id']! as String,
      handle = json['handle']! as String;

  /// The principal ID.
  final String id;

  /// The handle, without `@`.
  final String handle;
}

/// What the app of a desktop E2E run needs of its stack: the address it types and the instance
/// whose data folder it fills. [E2eWorld] is the world of the chat flows; the terminal flows of
/// brief M4-38 take theirs from the client stack of the terminal (`terminal_stack.dart`).
abstract interface class E2eTarget {
  /// The address the app types: the server's public URL.
  Uri get server;

  /// The instance ID.
  String get instanceId;
}

/// The world of the run.
final class E2eWorld implements E2eTarget {
  E2eWorld._(Map<String, Object?> json, this.caPem)
    : server = Uri.parse(json['server_url']! as String),
      instanceId = json['instance_id']! as String,
      alex = E2ePerson._(json['alex']! as Map<String, Object?>),
      sam = E2ePerson._(json['sam']! as Map<String, Object?>),
      spaceId = json['space_id']! as String,
      generalId = json['general_id']! as String,
      generalThreadId = json['general_thread_id']! as String,
      forumId = json['forum_id']! as String,
      agent = E2eAgentRef._(json['agent']! as Map<String, Object?>),
      streamAgent = E2eAgentRef._(
        json['stream_agent']! as Map<String, Object?>,
      );

  /// Parses e2e.json and the PEM of the test CA.
  factory E2eWorld.parse(String json, List<int> caPem) =>
      E2eWorld._(jsonDecode(json) as Map<String, Object?>, caPem);

  /// The world of the defines `E2E_CONFIG` and `E2E_CA`; throws a [StateError] naming the
  /// runner when they are missing (P-09: a run without a stack fails, it is never skipped).
  factory E2eWorld.fromEnvironment() {
    const config = String.fromEnvironment('E2E_CONFIG');
    const ca = String.fromEnvironment('E2E_CA');
    if (config.isEmpty || ca.isEmpty) {
      throw StateError(
        'E2E_CONFIG and E2E_CA are not defined; run the suite with make e2e-desktop or '
        'make perf-desktop (infra/compose/e2e.sh), never on its own',
      );
    }
    return E2eWorld.parse(
      utf8.decode(base64.decode(config)),
      base64.decode(ca),
    );
  }

  /// The address the app types: server.public_url, `https://localhost:<port>`.
  @override
  final Uri server;

  /// The instance ID.
  @override
  final String instanceId;

  /// Alex, the first owner.
  final E2ePerson alex;

  /// Sam, a person with `trigger` on the space.
  final E2ePerson sam;

  /// The space Support.
  final String spaceId;

  /// The text channel general.
  final String generalId;

  /// The main thread of general.
  final String generalThreadId;

  /// The forum requests.
  final String forumId;

  /// support-agent: a whole run with follow-up and approval card.
  final E2eAgentRef agent;

  /// stream-agent: a stream of patches, for the performance level.
  final E2eAgentRef streamAgent;

  /// The PEM of the test CA the TLS front's certificate comes from.
  final List<int> caPem;
}
