// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_definition_mcp_server.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentDefinitionMcpServerTransportEnum
_$agentDefinitionMcpServerTransportEnum_http =
    const AgentDefinitionMcpServerTransportEnum._('http');
const AgentDefinitionMcpServerTransportEnum
_$agentDefinitionMcpServerTransportEnum_stdio =
    const AgentDefinitionMcpServerTransportEnum._('stdio');
const AgentDefinitionMcpServerTransportEnum
_$agentDefinitionMcpServerTransportEnum_unknownDefaultOpenApi =
    const AgentDefinitionMcpServerTransportEnum._('unknownDefaultOpenApi');

AgentDefinitionMcpServerTransportEnum
_$agentDefinitionMcpServerTransportEnumValueOf(String name) {
  switch (name) {
    case 'http':
      return _$agentDefinitionMcpServerTransportEnum_http;
    case 'stdio':
      return _$agentDefinitionMcpServerTransportEnum_stdio;
    case 'unknownDefaultOpenApi':
      return _$agentDefinitionMcpServerTransportEnum_unknownDefaultOpenApi;
    default:
      return _$agentDefinitionMcpServerTransportEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentDefinitionMcpServerTransportEnum>
_$agentDefinitionMcpServerTransportEnumValues =
    BuiltSet<AgentDefinitionMcpServerTransportEnum>(
      const <AgentDefinitionMcpServerTransportEnum>[
        _$agentDefinitionMcpServerTransportEnum_http,
        _$agentDefinitionMcpServerTransportEnum_stdio,
        _$agentDefinitionMcpServerTransportEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<AgentDefinitionMcpServerTransportEnum>
_$agentDefinitionMcpServerTransportEnumSerializer =
    _$AgentDefinitionMcpServerTransportEnumSerializer();

class _$AgentDefinitionMcpServerTransportEnumSerializer
    implements PrimitiveSerializer<AgentDefinitionMcpServerTransportEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'http': 'http',
    'stdio': 'stdio',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'http': 'http',
    'stdio': 'stdio',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    AgentDefinitionMcpServerTransportEnum,
  ];
  @override
  final String wireName = 'AgentDefinitionMcpServerTransportEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionMcpServerTransportEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentDefinitionMcpServerTransportEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentDefinitionMcpServerTransportEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentDefinitionMcpServer extends AgentDefinitionMcpServer {
  @override
  final String name;
  @override
  final AgentDefinitionMcpServerTransportEnum transport;
  @override
  final String? url;
  @override
  final BuiltList<AgentDefinitionEntry>? headers;
  @override
  final String? command;
  @override
  final String? package;
  @override
  final BuiltList<String>? args;
  @override
  final BuiltList<AgentDefinitionEntry>? env;

  factory _$AgentDefinitionMcpServer([
    void Function(AgentDefinitionMcpServerBuilder)? updates,
  ]) => (AgentDefinitionMcpServerBuilder()..update(updates))._build();

  _$AgentDefinitionMcpServer._({
    required this.name,
    required this.transport,
    this.url,
    this.headers,
    this.command,
    this.package,
    this.args,
    this.env,
  }) : super._();
  @override
  AgentDefinitionMcpServer rebuild(
    void Function(AgentDefinitionMcpServerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentDefinitionMcpServerBuilder toBuilder() =>
      AgentDefinitionMcpServerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentDefinitionMcpServer &&
        name == other.name &&
        transport == other.transport &&
        url == other.url &&
        headers == other.headers &&
        command == other.command &&
        package == other.package &&
        args == other.args &&
        env == other.env;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, transport.hashCode);
    _$hash = $jc(_$hash, url.hashCode);
    _$hash = $jc(_$hash, headers.hashCode);
    _$hash = $jc(_$hash, command.hashCode);
    _$hash = $jc(_$hash, package.hashCode);
    _$hash = $jc(_$hash, args.hashCode);
    _$hash = $jc(_$hash, env.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentDefinitionMcpServer')
          ..add('name', name)
          ..add('transport', transport)
          ..add('url', url)
          ..add('headers', headers)
          ..add('command', command)
          ..add('package', package)
          ..add('args', args)
          ..add('env', env))
        .toString();
  }
}

class AgentDefinitionMcpServerBuilder
    implements
        Builder<AgentDefinitionMcpServer, AgentDefinitionMcpServerBuilder> {
  _$AgentDefinitionMcpServer? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  AgentDefinitionMcpServerTransportEnum? _transport;
  AgentDefinitionMcpServerTransportEnum? get transport => _$this._transport;
  set transport(AgentDefinitionMcpServerTransportEnum? transport) =>
      _$this._transport = transport;

  String? _url;
  String? get url => _$this._url;
  set url(String? url) => _$this._url = url;

  ListBuilder<AgentDefinitionEntry>? _headers;
  ListBuilder<AgentDefinitionEntry> get headers =>
      _$this._headers ??= ListBuilder<AgentDefinitionEntry>();
  set headers(ListBuilder<AgentDefinitionEntry>? headers) =>
      _$this._headers = headers;

  String? _command;
  String? get command => _$this._command;
  set command(String? command) => _$this._command = command;

  String? _package;
  String? get package => _$this._package;
  set package(String? package) => _$this._package = package;

  ListBuilder<String>? _args;
  ListBuilder<String> get args => _$this._args ??= ListBuilder<String>();
  set args(ListBuilder<String>? args) => _$this._args = args;

  ListBuilder<AgentDefinitionEntry>? _env;
  ListBuilder<AgentDefinitionEntry> get env =>
      _$this._env ??= ListBuilder<AgentDefinitionEntry>();
  set env(ListBuilder<AgentDefinitionEntry>? env) => _$this._env = env;

  AgentDefinitionMcpServerBuilder() {
    AgentDefinitionMcpServer._defaults(this);
  }

  AgentDefinitionMcpServerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _transport = $v.transport;
      _url = $v.url;
      _headers = $v.headers?.toBuilder();
      _command = $v.command;
      _package = $v.package;
      _args = $v.args?.toBuilder();
      _env = $v.env?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentDefinitionMcpServer other) {
    _$v = other as _$AgentDefinitionMcpServer;
  }

  @override
  void update(void Function(AgentDefinitionMcpServerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentDefinitionMcpServer build() => _build();

  _$AgentDefinitionMcpServer _build() {
    _$AgentDefinitionMcpServer _$result;
    try {
      _$result =
          _$v ??
          _$AgentDefinitionMcpServer._(
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'AgentDefinitionMcpServer',
              'name',
            ),
            transport: BuiltValueNullFieldError.checkNotNull(
              transport,
              r'AgentDefinitionMcpServer',
              'transport',
            ),
            url: url,
            headers: _headers?.build(),
            command: command,
            package: package,
            args: _args?.build(),
            env: _env?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'headers';
        _headers?.build();

        _$failedField = 'args';
        _args?.build();
        _$failedField = 'env';
        _env?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentDefinitionMcpServer',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
