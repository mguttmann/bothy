// SPDX-License-Identifier: Apache-2.0

/// The contract checks of the fake server (brief M4-17 Design note 5): every request body it
/// receives and every response body and gateway frame it sends is validated against the
/// schemas of `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml`.
library;

import 'dart:convert';
import 'dart:io';

import 'package:yaml/yaml.dart';

part 'contract_documents.g.dart';

/// One place where a JSON value breaks its schema.
///
/// [pointer] is a JSON Pointer (RFC 6901) into the checked value (`""` for the root), the
/// form the server uses in `FieldError.pointer` (06 5).
final class ContractViolation {
  /// Creates a violation at [pointer] with [code] (the `FieldError.code` of 06 5) and a short
  /// [detail] that never repeats the value.
  const ContractViolation(this.pointer, this.code, this.detail);

  /// Where the violation is.
  final String pointer;

  /// The rule that failed: `required`, `unknown_member`, `invalid_type`, `invalid_format`,
  /// `invalid_enum`, `too_short`, `too_long`, `too_small`, `too_large`, `too_few_items`,
  /// `too_many_items`, `pattern_mismatch` or `invalid`.
  final String code;

  /// What was expected.
  final String detail;

  @override
  String toString() => '${pointer.isEmpty ? '/' : pointer}: $code ($detail)';
}

/// Thrown when the fake itself would send something that breaks the contract: a programming
/// error of the fake, never of the client under test.
final class ContractError implements Exception {
  /// Creates the error for [what] (for example `response createMessage 201`).
  const ContractError(this.what, this.violations);

  /// The checked message.
  final String what;

  /// What is wrong.
  final List<ContractViolation> violations;

  @override
  String toString() => 'ContractError($what: ${violations.join('; ')})';
}

/// The facts of one REST operation of `docs/api/openapi.yaml`.
final class ContractOperation {
  /// Creates the facts.
  const ContractOperation({
    required this.operationId,
    required this.method,
    required this.path,
    required this.secured,
    required this.requestSchema,
    required this.responseSchemas,
    this.authzAction,
  });

  /// The `operationId`.
  final String operationId;

  /// The HTTP method in upper case.
  final String method;

  /// The path template below `/api/v1`, for example `/threads/{thread_id}/messages`.
  final String path;

  /// Whether the operation needs a token (`security` is not `[]`).
  final bool secured;

  /// The schema name of the JSON request body, or null.
  final String? requestSchema;

  /// The schema name of the JSON body per status (`201`, `4XX`, `default`); null for a
  /// response without body.
  final Map<String, String?> responseSchemas;

  /// The authorization action of `x-authz-action` (05), or null when the operation names
  /// none.
  final String? authzAction;
}

/// The API documents of the repository, loaded once per isolate.
///
/// Validates the JSON Schema subset the documents use: `type`, `required`, `properties`,
/// `additionalProperties`, `enum`, `items`, `$ref`, `allOf`, `oneOf` with a discriminator,
/// `format` for `uuid` and `date-time`, and the bounds `minimum`, `maximum`, `minLength`,
/// `maxLength`, `minItems`, `maxItems` and `pattern`. Every other keyword is ignored. JSON
/// `null` never validates, because the API has no `null` (ADR 0053).
///
/// Example:
/// ```dart
/// final contract = ContractSchemas.load();
/// final problems = contract.validate({'id': 'x'}, 'Thread');
/// // [/id: invalid_format (uuid), /channel_id: required (...), ...]
/// ```
final class ContractSchemas {
  ContractSchemas._(this._openapi, Map<String, Object?> asyncapi) {
    _indexOperations();
    _indexMessages(asyncapi);
  }

  static ContractSchemas? _loaded;

  final Map<String, Object?> _openapi;
  final Map<String, ContractOperation> _operations = {};
  final Map<String, String> _serverOps = {};
  final Map<String, String> _clientOps = {};
  final Map<String, String> _events = {};

  /// Loads `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` of the repository that holds
  /// [from] (default: the working directory), walking up to the folder that has `docs/api`.
  /// Without such a folder (an app under test in the macOS app sandbox, whose working
  /// directory is `/` and which cannot read the repository) it uses [embedded].
  ///
  /// The documents are read once per isolate and cached.
  static ContractSchemas load({Directory? from}) =>
      _loaded ??= _fromFiles(from ?? Directory.current) ?? embedded();

  /// The copy of both documents compiled into this library (`contract_documents.g.dart`,
  /// written by `make contract-documents`; `make codegen-check` keeps it equal to `docs/api`,
  /// docs/DECISIONS.md D-M4-18-17). Decoded once per isolate.
  static ContractSchemas embedded() => _embedded ??= ContractSchemas._(
    _decode(_openapiGzip),
    _decode(_asyncapiGzip),
  );

  static ContractSchemas? _embedded;

  static ContractSchemas? _fromFiles(Directory from) {
    var dir = from.absolute;
    while (true) {
      final openapi = File('${dir.path}/docs/api/openapi.yaml');
      final asyncapi = File('${dir.path}/docs/api/asyncapi.yaml');
      if (openapi.existsSync() && asyncapi.existsSync()) {
        return ContractSchemas._(
          _parse(openapi.readAsStringSync()),
          _parse(asyncapi.readAsStringSync()),
        );
      }
      final parent = dir.parent;
      if (parent.path == dir.path) return null;
      dir = parent;
    }
  }

  static Map<String, Object?> _decode(List<String> chunks) =>
      _parse(utf8.decode(gzip.decode(base64.decode(chunks.join()))));

  static Map<String, Object?> _parse(String yaml) =>
      _plain(loadYaml(yaml)) as Map<String, Object?>;

  /// The operation [operationId]; throws an [ArgumentError] for an unknown ID.
  ContractOperation operation(String operationId) =>
      _operations[operationId] ??
      (throw ArgumentError.value(
        operationId,
        'operationId',
        'not in openapi.yaml',
      ));

  /// Every operation of `openapi.yaml`.
  Iterable<ContractOperation> get operations => _operations.values;

  /// Validates [value] against the schema [schemaName] of `components.schemas`.
  List<ContractViolation> validate(Object? value, String schemaName) {
    final out = <ContractViolation>[];
    _check(value, {r'$ref': '#/components/schemas/$schemaName'}, '', out);
    return out;
  }

  /// Validates the request [body] of [operationId]; an operation without a JSON body accepts
  /// only an empty one.
  List<ContractViolation> request(String operationId, Object? body) {
    final schema = operation(operationId).requestSchema;
    if (schema == null) {
      return body == null
          ? const []
          : const [ContractViolation('', 'invalid', 'no body expected')];
    }
    if (body == null) {
      return const [ContractViolation('', 'required', 'a JSON body')];
    }
    return validate(body, schema);
  }

  /// Validates the response [body] of [operationId] with [status]; a status the operation does
  /// not list is a violation.
  List<ContractViolation> response(
    String operationId,
    int status,
    Object? body,
  ) {
    final responses = operation(operationId).responseSchemas;
    final key = responses.containsKey('$status')
        ? '$status'
        : responses.containsKey('${status ~/ 100}XX')
        ? '${status ~/ 100}XX'
        : responses.containsKey('default')
        ? 'default'
        : null;
    if (key == null) {
      return [ContractViolation('', 'invalid', 'status $status not listed')];
    }
    final schema = responses[key];
    if (schema == null) {
      return body == null
          ? const []
          : const [ContractViolation('', 'invalid', 'no body expected')];
    }
    return validate(body, schema);
  }

  /// Validates a gateway frame `{op, d}` the server sends; a `dispatch` is checked against
  /// the event schema of its `type` (asyncapi `event.<type>`).
  List<ContractViolation> serverFrame(Map<String, Object?> frame) =>
      _frame(frame, _serverOps, dispatch: true);

  /// Validates a gateway frame `{op, d}` a client sends.
  List<ContractViolation> clientFrame(Map<String, Object?> frame) =>
      _frame(frame, _clientOps, dispatch: false);

  List<ContractViolation> _frame(
    Map<String, Object?> frame,
    Map<String, String> ops, {
    required bool dispatch,
  }) {
    final out = <ContractViolation>[];
    for (final key in frame.keys) {
      if (key != 'op' && key != 'd') {
        out.add(ContractViolation('/$key', 'unknown_member', 'only op and d'));
      }
    }
    final op = frame['op'];
    if (op is! String) {
      return [...out, const ContractViolation('/op', 'required', 'a string')];
    }
    final d = frame['d'];
    String? schema;
    if (dispatch && op == 'dispatch') {
      final type = d is Map ? d['type'] : null;
      schema = type is String ? _events[type] : null;
      if (schema == null) {
        return [
          ...out,
          ContractViolation('/d/type', 'invalid_enum', 'a known event type'),
        ];
      }
    } else {
      schema = ops[op];
      if (schema == null) {
        return [...out, ContractViolation('/op', 'invalid_enum', 'a known op')];
      }
    }
    final inner = <ContractViolation>[];
    _check(d, {r'$ref': '#/components/schemas/$schema'}, '/d', inner);
    return [...out, ...inner];
  }

  // -------------------------------------------------------------------------------------------
  // Index

  void _indexOperations() {
    final paths = _openapi['paths'] as Map<String, Object?>? ?? const {};
    for (final MapEntry(key: path, value: item) in paths.entries) {
      if (item is! Map<String, Object?>) continue;
      for (final MapEntry(key: method, value: op) in item.entries) {
        if (op is! Map<String, Object?>) continue;
        final id = op['operationId'];
        if (id is! String) continue;
        final security = op['security'];
        _operations[id] = ContractOperation(
          operationId: id,
          method: method.toUpperCase(),
          path: path,
          secured: !(security is List && security.isEmpty),
          requestSchema: _bodySchema(op['requestBody']),
          authzAction: op['x-authz-action'] as String?,
          responseSchemas: {
            for (final MapEntry(:key, :value)
                in (op['responses'] as Map<String, Object?>? ?? const {})
                    .entries)
              key: _bodySchema(value),
          },
        );
      }
    }
  }

  /// The schema name of the JSON content of a request body or response (following a
  /// `$ref` into `components`), or null when it has no JSON content.
  String? _bodySchema(Object? node) {
    var n = node;
    if (n is Map<String, Object?> && n[r'$ref'] is String) {
      n = _resolve(n[r'$ref']! as String);
    }
    if (n is! Map<String, Object?>) return null;
    final content = n['content'];
    if (content is! Map<String, Object?>) return null;
    for (final type in ['application/json', 'application/problem+json']) {
      final media = content[type];
      if (media is Map<String, Object?>) {
        final schema = media['schema'];
        if (schema is Map<String, Object?> && schema[r'$ref'] is String) {
          return (schema[r'$ref']! as String).split('/').last;
        }
      }
    }
    return null;
  }

  void _indexMessages(Map<String, Object?> asyncapi) {
    final components =
        asyncapi['components'] as Map<String, Object?>? ?? const {};
    final messages =
        components['messages'] as Map<String, Object?>? ?? const {};
    final operations =
        asyncapi['operations'] as Map<String, Object?>? ?? const {};
    // Which ops the client sends: the messages of `receive` operations of the gateway.
    final received = <String>{};
    for (final op in operations.values) {
      if (op is! Map<String, Object?> || op['action'] != 'receive') continue;
      final channel = (op['channel'] as Map<String, Object?>?)?[r'$ref'];
      if (channel != '#/channels/gateway') continue;
      for (final m in (op['messages'] as List<Object?>? ?? const [])) {
        final ref = (m as Map<String, Object?>?)?[r'$ref'];
        if (ref is String) received.add(ref.split('/').last);
      }
    }
    for (final MapEntry(key: name, value: message) in messages.entries) {
      if (message is! Map<String, Object?>) continue;
      final payload = message['payload'] as Map<String, Object?>?;
      final d = (payload?['properties'] as Map<String, Object?>?)?['d'];
      final ref = (d as Map<String, Object?>?)?[r'$ref'];
      if (ref is! String || !ref.contains('#/components/schemas/')) continue;
      final schema = ref.split('/').last;
      if (name.startsWith('event.')) {
        _events[name.substring('event.'.length)] = schema;
      } else if (name.startsWith('op.')) {
        final op = name.substring('op.'.length);
        final isClient = received.any((r) => r.endsWith(name) || r == name);
        (isClient ? _clientOps : _serverOps)[op] = schema;
      }
    }
  }

  Object? _resolve(String ref) {
    if (!ref.startsWith('#/')) {
      throw ArgumentError.value(ref, 'ref', 'only local references');
    }
    Object? node = _openapi;
    for (final part in ref.substring(2).split('/')) {
      node =
          (node as Map<String, Object?>?)?[part
              .replaceAll('~1', '/')
              .replaceAll('~0', '~')];
    }
    if (node == null) throw ArgumentError.value(ref, 'ref', 'does not resolve');
    return node;
  }

  // -------------------------------------------------------------------------------------------
  // Validation

  static final RegExp _uuid = RegExp(
    r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
  );
  static final RegExp _dateTime = RegExp(
    r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$',
  );

  void _check(
    Object? value,
    Map<String, Object?> schema,
    String at,
    List<ContractViolation> out,
  ) {
    final ref = schema[r'$ref'];
    if (ref is String) {
      _check(value, _resolve(ref)! as Map<String, Object?>, at, out);
      return;
    }
    final allOf = schema['allOf'];
    if (allOf is List<Object?>) {
      for (final part in allOf) {
        _check(value, part! as Map<String, Object?>, at, out);
      }
    }
    final oneOf = schema['oneOf'];
    if (oneOf is List<Object?>) _oneOf(value, schema, oneOf, at, out);
    final type = schema['type'];
    if (type != null && !_typeMatches(value, type)) {
      out.add(ContractViolation(at, 'invalid_type', '$type'));
      return;
    }
    final allowed = schema['enum'];
    if (allowed is List<Object?> && !allowed.contains(value)) {
      out.add(
        ContractViolation(at, 'invalid_enum', 'one of the listed values'),
      );
    }
    switch (value) {
      case final String s:
        _string(s, schema, at, out);
      case final num n:
        _number(n, schema, at, out);
      case final List<Object?> list:
        _array(list, schema, at, out);
      case final Map<String, Object?> map:
        _object(map, schema, at, out);
      case null when type == null && schema.isNotEmpty:
        out.add(
          ContractViolation(at, 'invalid_type', 'no JSON null (ADR 0053)'),
        );
    }
  }

  void _oneOf(
    Object? value,
    Map<String, Object?> schema,
    List<Object?> options,
    String at,
    List<ContractViolation> out,
  ) {
    final discriminator = schema['discriminator'];
    if (discriminator is Map<String, Object?> &&
        value is Map<String, Object?>) {
      final name = discriminator['propertyName']! as String;
      final mapping =
          discriminator['mapping'] as Map<String, Object?>? ?? const {};
      final ref = mapping[value[name]];
      if (ref is! String) {
        out.add(
          ContractViolation(
            '$at/$name',
            'invalid_enum',
            'a mapped discriminator',
          ),
        );
        return;
      }
      _check(value, {r'$ref': ref}, at, out);
      return;
    }
    var matches = 0;
    for (final option in options) {
      final inner = <ContractViolation>[];
      _check(value, option! as Map<String, Object?>, at, inner);
      if (inner.isEmpty) matches++;
    }
    if (matches != 1) {
      out.add(
        ContractViolation(
          at,
          'invalid',
          'exactly one oneOf option ($matches matched)',
        ),
      );
    }
  }

  static bool _typeMatches(Object? value, Object type) {
    if (type is List<Object?>) return type.any((t) => _typeMatches(value, t!));
    return switch (type) {
      'string' => value is String,
      'integer' =>
        value is int || (value is double && value == value.truncateToDouble()),
      'number' => value is num,
      'boolean' => value is bool,
      'object' => value is Map<String, Object?>,
      'array' => value is List<Object?>,
      'null' => false,
      _ => true,
    };
  }

  void _string(
    String s,
    Map<String, Object?> schema,
    String at,
    List<ContractViolation> out,
  ) {
    switch (schema['format']) {
      case 'uuid' when !_uuid.hasMatch(s):
        out.add(ContractViolation(at, 'invalid_format', 'uuid'));
      case 'date-time'
          when !_dateTime.hasMatch(s) || DateTime.tryParse(s) == null:
        out.add(ContractViolation(at, 'invalid_format', 'date-time'));
    }
    final min = schema['minLength'];
    if (min is int && s.length < min) {
      out.add(ContractViolation(at, 'too_short', 'at least $min characters'));
    }
    final max = schema['maxLength'];
    if (max is int && s.length > max) {
      out.add(ContractViolation(at, 'too_long', 'at most $max characters'));
    }
    final pattern = schema['pattern'];
    if (pattern is String && !RegExp(pattern).hasMatch(s)) {
      out.add(
        ContractViolation(at, 'pattern_mismatch', 'the pattern of the schema'),
      );
    }
  }

  void _number(
    num n,
    Map<String, Object?> schema,
    String at,
    List<ContractViolation> out,
  ) {
    final min = schema['minimum'];
    if (min is num && n < min) {
      out.add(ContractViolation(at, 'too_small', 'at least $min'));
    }
    final max = schema['maximum'];
    if (max is num && n > max) {
      out.add(ContractViolation(at, 'too_large', 'at most $max'));
    }
  }

  void _array(
    List<Object?> list,
    Map<String, Object?> schema,
    String at,
    List<ContractViolation> out,
  ) {
    final min = schema['minItems'];
    if (min is int && list.length < min) {
      out.add(ContractViolation(at, 'too_few_items', 'at least $min items'));
    }
    final max = schema['maxItems'];
    if (max is int && list.length > max) {
      out.add(ContractViolation(at, 'too_many_items', 'at most $max items'));
    }
    final items = schema['items'];
    if (items is Map<String, Object?>) {
      for (var i = 0; i < list.length; i++) {
        _check(list[i], items, '$at/$i', out);
      }
    }
  }

  void _object(
    Map<String, Object?> map,
    Map<String, Object?> schema,
    String at,
    List<ContractViolation> out,
  ) {
    final properties =
        schema['properties'] as Map<String, Object?>? ?? const {};
    for (final name in (schema['required'] as List<Object?>? ?? const [])) {
      if (!map.containsKey(name)) {
        out.add(
          ContractViolation(
            '$at/${_escape(name! as String)}',
            'required',
            'present',
          ),
        );
      }
    }
    final extra = schema['additionalProperties'];
    for (final MapEntry(:key, :value) in map.entries) {
      final pointer = '$at/${_escape(key)}';
      final property = properties[key];
      if (property is Map<String, Object?>) {
        _check(value, property, pointer, out);
      } else if (extra == false) {
        out.add(
          ContractViolation(pointer, 'unknown_member', 'not in the schema'),
        );
      } else if (extra is Map<String, Object?>) {
        _check(value, extra, pointer, out);
      }
    }
  }

  static String _escape(String name) =>
      name.replaceAll('~', '~0').replaceAll('/', '~1');

  /// [node] of `package:yaml` as plain maps and lists.
  static Object? _plain(Object? node) => switch (node) {
    final YamlMap m => <String, Object?>{
      for (final e in m.entries) '${e.key}': _plain(e.value),
    },
    final YamlList l => [for (final v in l) _plain(v)],
    _ => node,
  };
}
