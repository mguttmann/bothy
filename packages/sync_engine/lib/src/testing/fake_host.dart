// SPDX-License-Identifier: Apache-2.0

/// A host of the terminal as the fake server stores it (docs/spec/24-terminal.md 3; brief
/// M4-37 Scope: fake handlers with fingerprint attempts, I6 checks, taken names and doctor
/// results that name session users).
library;

/// The JSON maps of the fake.
typedef HostJson = Map<String, Object?>;

/// One host: its state, keys, declaration, reported names, valid assignments and every name
/// ever given (ruling T12: a name stays bound to its person on the host).
final class FakeHost {
  /// Creates a host in `pairing`.
  FakeHost({
    required this.id,
    required this.name,
    required this.createdAt,
    this.assignedUserId,
  });

  /// The host ID.
  final String id;

  /// The name.
  String name;

  /// `pairing`, `confirming`, `active`, `disabled` or `retired`.
  String state = 'pairing';

  /// `owner` or `duplicate_connection` with `disabled`, else null.
  String? disabledReason;

  /// The assignee, or null.
  String? assignedUserId;

  /// `none`, `personal` or `organization`.
  String llmCredentials = 'none';

  /// The holder of a personal login, or null.
  String? credentialHolderId;

  /// The owner's answer to the server host question.
  bool declaredServerHost = false;

  /// The fingerprint of the stored keys, or null.
  String? fingerprint;

  /// The key generation.
  int keyGeneration = 0;

  /// Failed confirmations since the last pairing.
  int failures = 0;

  /// The names the host reported, or null before pairing.
  List<String>? reported;

  /// The doctor entries (`HostDoctorCheck`).
  List<HostJson> doctor = [];

  /// Valid assignments: person to name, with the time of the assignment.
  final Map<String, (String, String)> sessionUsers = {};

  /// Every name ever given on this host, with its person.
  final Map<String, String> taken = {};

  /// Whether the control connection is open.
  bool connected = false;

  /// When the host was created.
  final String createdAt;

  /// The version.
  int version = 1;

  /// The pairing code that is valid, or null.
  String? pairingCode;

  /// The host as the API returns it (`Host`).
  HostJson get json => {
    'id': id,
    'name': name,
    'state': state,
    'disabled_reason': ?disabledReason,
    if (reported != null) 'platform': 'linux',
    'assigned_user_id': ?assignedUserId,
    'llm_credentials': llmCredentials,
    'credential_holder_id': ?credentialHolderId,
    'declared_server_host': declaredServerHost,
    if (reported != null) 'reported_is_server_host': false,
    'key_fingerprint': ?fingerprint,
    'key_generation': keyGeneration,
    'profiles': [
      if (reported != null)
        {'name': 'shell', 'display_name': 'Shell', 'kind': 'shell'},
    ],
    'doctor': doctor,
    if (sessionUsers.isNotEmpty)
      'session_users': [
        for (final MapEntry(key: user, value: (name, at))
            in sessionUsers.entries)
          {'user_id': user, 'os_user': name, 'created_at': at},
      ],
    if (reported != null)
      'reported_session_users': [
        for (final n in reported!) {'os_user': n, 'workdirs': <Object?>[]},
      ],
    'connected': connected,
    if (reported != null) 'host_version': '1.0.0',
    'created_at': createdAt,
    'version': version,
  };
}
