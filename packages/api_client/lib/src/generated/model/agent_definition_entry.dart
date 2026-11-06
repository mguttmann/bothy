//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_entry.g.dart';

/// A header or an environment entry with exactly one of `value` and `secret_ref`. A header named `Authorization`, `Cookie` or `Proxy-Authorization`, and every entry whose name contains `KEY`, `TOKEN`, `SECRET`, `PASSWORD`, `PASSWD`, `CREDENTIAL`, `AUTH` or `COOKIE` (any case), needs `secret_ref` (`secret_value_not_allowed`). The server never resolves a `secret_ref`.
///
/// Properties:
/// * [name]
/// * [value] - A plain value, shown in the preview.
/// * [secretRef] - A key of the secret provider of the hosting runner, `env:<NAME>` or `file:<absolute path>`.
@BuiltValue()
abstract class AgentDefinitionEntry
    implements Built<AgentDefinitionEntry, AgentDefinitionEntryBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  /// A plain value, shown in the preview.
  @BuiltValueField(wireName: r'value')
  String? get value;

  /// A key of the secret provider of the hosting runner, `env:<NAME>` or `file:<absolute path>`.
  @BuiltValueField(wireName: r'secret_ref')
  String? get secretRef;

  AgentDefinitionEntry._();

  factory AgentDefinitionEntry([void updates(AgentDefinitionEntryBuilder b)]) =
      _$AgentDefinitionEntry;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionEntryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionEntry> get serializer =>
      _$AgentDefinitionEntrySerializer();
}

class _$AgentDefinitionEntrySerializer
    implements PrimitiveSerializer<AgentDefinitionEntry> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionEntry,
    _$AgentDefinitionEntry,
  ];

  @override
  final String wireName = r'AgentDefinitionEntry';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionEntry object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    if (object.value != null) {
      yield r'value';
      yield serializers.serialize(
        object.value,
        specifiedType: const FullType(String),
      );
    }
    if (object.secretRef != null) {
      yield r'secret_ref';
      yield serializers.serialize(
        object.secretRef,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionEntry object, {
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
    required AgentDefinitionEntryBuilder result,
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
        case r'value':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.value = valueDes;
          break;
        case r'secret_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.secretRef = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinitionEntry deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionEntryBuilder();
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
