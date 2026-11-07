// SPDX-License-Identifier: Apache-2.0

/// The draft of "New agent" as the preview edits it (docs/spec/01-product.md 6.20 item 3;
/// docs/spec/26-builtin-agents.md 4.2, 8.2): the agent's identity, the definition of schema
/// `agent_definition.v1`, the tool class and the channel to create.
///
/// A template, an interview draft and the empty form all become one [NewAgentDraft] (one
/// preview for every way in, 26 8.2). [NewAgentDraft.definitionJson] writes the definition
/// with exactly the members of 26 4.2, optional members only when they carry a value, so the
/// server's closed schema sees what the preview shows and nothing else.
library;

import 'dart:convert';

/// A tool class of 07 2.11 (`chat` < `scoped` < `host`).
enum AgentToolClass {
  /// Answers only; loads no MCP server (26 4.4 item 3).
  chat('chat'),

  /// The runner's allow list narrowed by the definition; only `http` MCP servers.
  scoped('scoped'),

  /// The rights of the host user; every MCP server, `stdio` included (SR2).
  host('host');

  const AgentToolClass(this.wire);

  /// The name on the wire.
  final String wire;

  /// The class of [wire]; `chat` for an unknown or missing value, the safe reading of a value
  /// this build does not know.
  static AgentToolClass of(String? wire) =>
      values.where((c) => c.wire == wire).firstOrNull ?? chat;

  /// Whether this class is above [other].
  bool isAbove(AgentToolClass other) => index > other.index;
}

/// A header of an `http` server or an environment entry of a `stdio` server: exactly one of
/// [value] and [secretRef] is used (26 4.2), chosen by [isSecret].
final class DraftEntry {
  /// Creates the entry.
  DraftEntry({
    this.name = '',
    this.value = '',
    this.secretRef = '',
    this.isSecret = false,
  });

  /// The entry of the API's [json] (`AgentDefinitionEntry`).
  factory DraftEntry.fromJson(Map<String, Object?> json) {
    final ref = json['secret_ref'] as String?;
    return DraftEntry(
      name: json['name'] as String? ?? '',
      value: json['value'] as String? ?? '',
      secretRef: ref ?? '',
      isSecret: ref != null,
    );
  }

  /// The header or variable name.
  String name;

  /// The plain value; the preview shows every plain value (26 4.3 item 5, T-BA-02).
  String value;

  /// The secret reference, for example `env:DOCS_MCP_TOKEN`; the server never resolves it.
  String secretRef;

  /// Whether the entry is a secret reference.
  bool isSecret;

  /// The `AgentDefinitionEntry` JSON.
  Map<String, Object?> toJson() => {
    'name': name,
    if (isSecret) 'secret_ref': secretRef else 'value': value,
  };
}

/// One MCP server of the draft (26 4.2 `mcp_servers[]`).
final class DraftMcpServer {
  /// Creates the server; [transport] is `http` or `stdio`.
  DraftMcpServer({
    this.name = '',
    this.transport = 'http',
    this.url = '',
    List<DraftEntry>? headers,
    this.command = '',
    this.package = '',
    List<String>? args,
    List<DraftEntry>? env,
  }) : headers = headers ?? [],
       args = args ?? [],
       env = env ?? [];

  /// The server of the API's [json] (`AgentDefinitionMcpServer`).
  factory DraftMcpServer.fromJson(Map<String, Object?> json) => DraftMcpServer(
    name: json['name'] as String? ?? '',
    transport: json['transport'] as String? ?? 'http',
    url: json['url'] as String? ?? '',
    headers: [for (final e in _maps(json['headers'])) DraftEntry.fromJson(e)],
    command: json['command'] as String? ?? '',
    package: json['package'] as String? ?? '',
    args: [
      for (final a in (json['args'] as List?) ?? const <Object?>[])
        if (a is String) a,
    ],
    env: [for (final e in _maps(json['env'])) DraftEntry.fromJson(e)],
  );

  /// The name, unique among the servers.
  String name;

  /// `http` or `stdio`.
  String transport;

  /// The `https` URL of an `http` server.
  String url;

  /// The headers of an `http` server.
  final List<DraftEntry> headers;

  /// The command of a `stdio` server, one of `agents.mcp_stdio_runners`.
  String command;

  /// The pinned package of a `stdio` server, `name@version`.
  String package;

  /// The arguments of a `stdio` server after the package.
  final List<String> args;

  /// The environment of a `stdio` server.
  final List<DraftEntry> env;

  /// Whether the server starts a local process (`stdio`), which needs the class `host`
  /// (26 4.4 item 4).
  bool get isStdio => transport == 'stdio';

  /// The entries of the server's transport.
  List<DraftEntry> get entries => isStdio ? env : headers;

  /// The secret references the server names, in order.
  List<String> get secretRefs => [
    for (final e in entries)
      if (e.isSecret && e.secretRef.isNotEmpty) e.secretRef,
  ];

  /// The `AgentDefinitionMcpServer` JSON; the members of the other transport stay out (a
  /// member of the other transport is `unknown_member`).
  Map<String, Object?> toJson() => isStdio
      ? {
          'name': name,
          'transport': 'stdio',
          'command': command,
          'package': package,
          if (args.isNotEmpty) 'args': [...args],
          if (env.isNotEmpty) 'env': [for (final e in env) e.toJson()],
        }
      : {
          'name': name,
          'transport': 'http',
          'url': url,
          if (headers.isNotEmpty)
            'headers': [for (final h in headers) h.toJson()],
        };
}

/// One skill of the draft (26 4.2 `skills[]`); extra [files] are kept as they came.
final class DraftSkill {
  /// Creates the skill.
  DraftSkill({
    this.name = '',
    this.description = '',
    this.body = '',
    List<Map<String, Object?>>? files,
  }) : files = files ?? [];

  /// The skill of the API's [json] (`AgentDefinitionSkill`).
  factory DraftSkill.fromJson(Map<String, Object?> json) => DraftSkill(
    name: json['name'] as String? ?? '',
    description: json['description'] as String? ?? '',
    body: json['body'] as String? ?? '',
    files: [for (final f in _maps(json['files'])) Map.of(f)],
  );

  /// The name, unique among the skills.
  String name;

  /// One line on what the skill does.
  String description;

  /// The Markdown body.
  String body;

  /// The extra files (`name`, `content`), written flat next to `SKILL.md`.
  final List<Map<String, Object?>> files;

  /// The `AgentDefinitionSkill` JSON.
  Map<String, Object?> toJson() => {
    'name': name,
    'description': description,
    'body': body,
    if (files.isNotEmpty) 'files': [for (final f in files) Map.of(f)],
  };
}

/// Where the draft came from (`origin.kind`, display only).
enum DraftOrigin {
  /// A template of 26 7.
  template,

  /// The interview of 26 6.
  interview,

  /// The empty form.
  manual,
}

/// The whole draft of the preview.
///
/// Example:
/// ```dart
/// final draft = NewAgentDraft.empty()
///   ..name = 'Notes'
///   ..handle = 'notes'
///   ..instructions = 'Keep the meeting notes of the team.';
/// final body = draft.definitionJson(); // the body of putAgentDefinition
/// ```
final class NewAgentDraft {
  NewAgentDraft._({
    required this.origin,
    this.templateSlug,
    this.templateVersion,
    this.requiredSecrets = const [],
  });

  /// The empty form (`origin.kind = manual`).
  factory NewAgentDraft.empty() => NewAgentDraft._(origin: DraftOrigin.manual);

  /// The draft of a template: its definition one to one (26 7.1), with the template's name and
  /// description as the agent's and its slug as the suggested handle.
  factory NewAgentDraft.fromTemplate({
    required String slug,
    required String name,
    required String description,
    required String version,
    required Map<String, Object?> definition,
    List<String> requiredSecrets = const [],
  }) =>
      NewAgentDraft._(
          origin: DraftOrigin.template,
          templateSlug: slug,
          templateVersion: version,
          requiredSecrets: requiredSecrets,
        )
        .._readDefinition(definition)
        ..name = name
        ..handle = slug
        ..description = description;

  /// The draft of the builder (`AgentDraft`, 26 6.1 item 2): the definition plus name, handle,
  /// description and the channel suggestion, whose space is matched by name in [spaceIdOf].
  factory NewAgentDraft.fromInterview(
    Map<String, Object?> draft, {
    String? Function(String spaceName)? spaceIdOf,
  }) {
    final d = NewAgentDraft._(origin: DraftOrigin.interview)
      .._readDefinition(draft)
      ..name = draft['name'] as String? ?? ''
      ..handle = draft['handle'] as String? ?? ''
      ..description = draft['description'] as String? ?? '';
    final channel = draft['channel_suggestion'];
    if (channel is Map) {
      d
        ..channelWanted = true
        ..channelName = channel['channel_name'] as String? ?? ''
        ..channelSpaceId = spaceIdOf?.call(
          channel['space_name'] as String? ?? '',
        );
    }
    return d;
  }

  /// Where the draft came from.
  final DraftOrigin origin;

  /// The template's slug, or null.
  final String? templateSlug;

  /// The template's version, or null.
  final String? templateVersion;

  /// The secret names the template needs on the runner host (`required_secrets`).
  final List<String> requiredSecrets;

  /// The display name.
  String name = '';

  /// The handle, `^[a-z0-9][a-z0-9._-]*$`.
  String handle = '';

  /// One line on what the agent does.
  String description = '';

  /// The Markdown that becomes `CLAUDE.md`.
  String instructions = '';

  /// The skills.
  final List<DraftSkill> skills = [];

  /// The MCP servers.
  final List<DraftMcpServer> servers = [];

  /// The model, empty for the runner's default model.
  String model = '';

  /// The tool class the owner picks; preselected from `tool_class_wish` (26 4.4 item 1).
  AgentToolClass toolClass = AgentToolClass.chat;

  /// The tool list that narrows `scoped`; null for no narrowing.
  List<String>? allowedTools;

  /// The cost cap per run in USD as typed, empty for no cap.
  String costCap = '';

  /// Whether a channel with the agent as operator is created.
  bool channelWanted = false;

  /// The space of the channel, or null.
  String? channelSpaceId;

  /// The channel's name.
  String channelName = '';

  void _readDefinition(Map<String, Object?> json) {
    instructions = json['instructions'] as String? ?? '';
    skills
      ..clear()
      ..addAll([for (final s in _maps(json['skills'])) DraftSkill.fromJson(s)]);
    servers
      ..clear()
      ..addAll([
        for (final s in _maps(json['mcp_servers'])) DraftMcpServer.fromJson(s),
      ]);
    final settings = json['settings'];
    if (settings is Map) {
      model = settings['model'] as String? ?? '';
      toolClass = AgentToolClass.of(settings['tool_class_wish'] as String?);
      final tools = settings['allowed_tools'];
      allowedTools = tools is List ? [for (final t in tools) '$t'] : null;
      final cap = settings['max_cost_usd_per_run'];
      costCap = cap is num ? _decimal(cap) : '';
    }
  }

  /// The cost cap as a number, null when empty; [double.nan] for text that is no finite
  /// number, so the preview can mark it.
  double? get costCapValue {
    final text = costCap.trim();
    if (text.isEmpty) return null;
    final value = double.tryParse(text);
    return value != null && value.isFinite ? value : double.nan;
  }

  /// Whether a server starts a local process, which needs `host` (26 4.4 item 4).
  bool get hasStdio => servers.any((s) => s.isStdio);

  /// The secret names to add on the runner host: every `secret_ref` of the servers and the
  /// template's `required_secrets`, each once, in order. Names only, never a value.
  List<String> get secretNames {
    final out = <String>[];
    void add(String name) {
      if (name.isNotEmpty && !out.contains(name)) out.add(name);
    }

    for (final s in servers) {
      s.secretRefs.forEach(add);
    }
    requiredSecrets.forEach(add);
    return out;
  }

  /// The body of `putAgentDefinition` (`agent_definition.v1`, 26 4.2).
  Map<String, Object?> definitionJson() {
    final cap = costCapValue;
    return {
      'instructions': instructions,
      'skills': [for (final s in skills) s.toJson()],
      'mcp_servers': [for (final s in servers) s.toJson()],
      'settings': {
        if (model.trim().isNotEmpty) 'model': model.trim(),
        'tool_class_wish': toolClass.wire,
        if (allowedTools case final tools?) 'allowed_tools': [...tools],
        if (cap != null && !cap.isNaN) 'max_cost_usd_per_run': cap,
      },
      'origin': {
        'kind': origin.name,
        'template_slug': ?templateSlug,
        'template_version': ?templateVersion,
      },
    };
  }
}

/// Whether two definition bodies are the same content.
///
/// Example: `sameDefinition(stored.json, draft.definitionJson())`.
bool sameDefinition(Map<String, Object?>? a, Map<String, Object?>? b) =>
    a != null && b != null && _canonical(a) == _canonical(b);

String _canonical(Object? value) {
  if (value is Map) {
    final keys = [for (final k in value.keys) '$k']..sort();
    return '{${[for (final k in keys) '${jsonEncode(k)}:${_canonical(value[k])}'].join(',')}}';
  }
  if (value is List) return '[${value.map(_canonical).join(',')}]';
  if (value is num) return jsonEncode(value.toDouble());
  return jsonEncode(value);
}

String _decimal(num n) {
  final text = n.toString();
  return text.endsWith('.0') ? text.substring(0, text.length - 2) : text;
}

List<Map<String, Object?>> _maps(Object? list) => [
  if (list is List)
    for (final item in list)
      if (item is Map) item.cast<String, Object?>(),
];
