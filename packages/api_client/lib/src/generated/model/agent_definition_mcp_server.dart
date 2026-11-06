//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_definition_entry.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_mcp_server.g.dart';

/// One MCP server. `http` takes `url` and optional `headers`; `stdio` takes `command`, `package`, optional `args` and `env`. A member of the other transport is `unknown_member`. The runner places `package` directly after `command` and every entry of `args` after it.
///
/// Properties:
/// * [name] - `^[a-z0-9][a-z0-9-]{0,63}$`, unique among the servers.
/// * [transport]
/// * [url] - `http`: an `https` URL with a host, without user information and without fragment.
/// * [headers] - `http`: headers, names unique without case.
/// * [command] - `stdio`: one of `agents.mcp_stdio_runners` (`mcp_command_not_allowed`).
/// * [package] - `stdio`: `<name>@<exact version>`, a semantic version without range, wildcard or tag (`mcp_package_unpinned`).
/// * [args] - `stdio`: arguments after the package.
/// * [env] - `stdio`: environment entries, names `^[A-Z_][A-Z0-9_]*$`, unique, never a name the runner sets itself (`env_name_reserved`).
@BuiltValue()
abstract class AgentDefinitionMcpServer
    implements
        Built<AgentDefinitionMcpServer, AgentDefinitionMcpServerBuilder> {
  /// `^[a-z0-9][a-z0-9-]{0,63}$`, unique among the servers.
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'transport')
  AgentDefinitionMcpServerTransportEnum get transport;
  // enum transportEnum {  http,  stdio,  };

  /// `http`: an `https` URL with a host, without user information and without fragment.
  @BuiltValueField(wireName: r'url')
  String? get url;

  /// `http`: headers, names unique without case.
  @BuiltValueField(wireName: r'headers')
  BuiltList<AgentDefinitionEntry>? get headers;

  /// `stdio`: one of `agents.mcp_stdio_runners` (`mcp_command_not_allowed`).
  @BuiltValueField(wireName: r'command')
  String? get command;

  /// `stdio`: `<name>@<exact version>`, a semantic version without range, wildcard or tag (`mcp_package_unpinned`).
  @BuiltValueField(wireName: r'package')
  String? get package;

  /// `stdio`: arguments after the package.
  @BuiltValueField(wireName: r'args')
  BuiltList<String>? get args;

  /// `stdio`: environment entries, names `^[A-Z_][A-Z0-9_]*$`, unique, never a name the runner sets itself (`env_name_reserved`).
  @BuiltValueField(wireName: r'env')
  BuiltList<AgentDefinitionEntry>? get env;

  AgentDefinitionMcpServer._();

  factory AgentDefinitionMcpServer([
    void updates(AgentDefinitionMcpServerBuilder b),
  ]) = _$AgentDefinitionMcpServer;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionMcpServerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionMcpServer> get serializer =>
      _$AgentDefinitionMcpServerSerializer();
}

class _$AgentDefinitionMcpServerSerializer
    implements PrimitiveSerializer<AgentDefinitionMcpServer> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionMcpServer,
    _$AgentDefinitionMcpServer,
  ];

  @override
  final String wireName = r'AgentDefinitionMcpServer';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionMcpServer object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'transport';
    yield serializers.serialize(
      object.transport,
      specifiedType: const FullType(AgentDefinitionMcpServerTransportEnum),
    );
    if (object.url != null) {
      yield r'url';
      yield serializers.serialize(
        object.url,
        specifiedType: const FullType(String),
      );
    }
    if (object.headers != null) {
      yield r'headers';
      yield serializers.serialize(
        object.headers,
        specifiedType: const FullType(BuiltList, [
          FullType(AgentDefinitionEntry),
        ]),
      );
    }
    if (object.command != null) {
      yield r'command';
      yield serializers.serialize(
        object.command,
        specifiedType: const FullType(String),
      );
    }
    if (object.package != null) {
      yield r'package';
      yield serializers.serialize(
        object.package,
        specifiedType: const FullType(String),
      );
    }
    if (object.args != null) {
      yield r'args';
      yield serializers.serialize(
        object.args,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.env != null) {
      yield r'env';
      yield serializers.serialize(
        object.env,
        specifiedType: const FullType(BuiltList, [
          FullType(AgentDefinitionEntry),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionMcpServer object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required AgentDefinitionMcpServerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'transport':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(
              AgentDefinitionMcpServerTransportEnum,
            ),
          ) as AgentDefinitionMcpServerTransportEnum;
          result.transport = valueDes;
          break;
        case r'url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.url = valueDes;
          break;
        case r'headers':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(AgentDefinitionEntry),
            ]),
          ) as BuiltList<AgentDefinitionEntry>?;
          if (valueDes == null) continue;
          result.headers.replace(valueDes);
          break;
        case r'command':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.command = valueDes;
          break;
        case r'package':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.package = valueDes;
          break;
        case r'args':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.args.replace(valueDes);
          break;
        case r'env':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(AgentDefinitionEntry),
            ]),
          ) as BuiltList<AgentDefinitionEntry>?;
          if (valueDes == null) continue;
          result.env.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinitionMcpServer deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionMcpServerBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}

class AgentDefinitionMcpServerTransportEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'http')
  static const AgentDefinitionMcpServerTransportEnum http =
      _$agentDefinitionMcpServerTransportEnum_http;
  @BuiltValueEnumConst(wireName: r'stdio')
  static const AgentDefinitionMcpServerTransportEnum stdio =
      _$agentDefinitionMcpServerTransportEnum_stdio;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentDefinitionMcpServerTransportEnum unknownDefaultOpenApi =
      _$agentDefinitionMcpServerTransportEnum_unknownDefaultOpenApi;

  static Serializer<AgentDefinitionMcpServerTransportEnum> get serializer =>
      _$agentDefinitionMcpServerTransportEnumSerializer;

  const AgentDefinitionMcpServerTransportEnum._(String name) : super(name);

  static BuiltSet<AgentDefinitionMcpServerTransportEnum> get values =>
      _$agentDefinitionMcpServerTransportEnumValues;
  static AgentDefinitionMcpServerTransportEnum valueOf(String name) =>
      _$agentDefinitionMcpServerTransportEnumValueOf(name);
}
