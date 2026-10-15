//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_create.g.dart';

/// AgentCreate
///
/// Properties:
/// * [handle]
/// * [displayName]
/// * [adapterKind]
/// * [description]
/// * [operatorConfig] - Free-form JSON object whose schema is named in the field description.
@BuiltValue()
abstract class AgentCreate implements Built<AgentCreate, AgentCreateBuilder> {
  @BuiltValueField(wireName: r'handle')
  String get handle;

  @BuiltValueField(wireName: r'display_name')
  String get displayName;

  @BuiltValueField(wireName: r'adapter_kind')
  String get adapterKind;

  @BuiltValueField(wireName: r'description')
  String? get description;

  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'operator_config')
  BuiltMap<String, JsonObject?>? get operatorConfig;

  AgentCreate._();

  factory AgentCreate([void updates(AgentCreateBuilder b)]) = _$AgentCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentCreate> get serializer => _$AgentCreateSerializer();
}

class _$AgentCreateSerializer implements PrimitiveSerializer<AgentCreate> {
  @override
  final Iterable<Type> types = const [AgentCreate, _$AgentCreate];

  @override
  final String wireName = r'AgentCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'handle';
    yield serializers.serialize(
      object.handle,
      specifiedType: const FullType(String),
    );
    yield r'display_name';
    yield serializers.serialize(
      object.displayName,
      specifiedType: const FullType(String),
    );
    yield r'adapter_kind';
    yield serializers.serialize(
      object.adapterKind,
      specifiedType: const FullType(String),
    );
    if (object.description != null) {
      yield r'description';
      yield serializers.serialize(
        object.description,
        specifiedType: const FullType(String),
      );
    }
    if (object.operatorConfig != null) {
      yield r'operator_config';
      yield serializers.serialize(
        object.operatorConfig,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentCreate object, {
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
    required AgentCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'handle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.handle = valueDes;
          break;
        case r'display_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.displayName = valueDes;
          break;
        case r'adapter_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.adapterKind = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.description = valueDes;
          break;
        case r'operator_config':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.operatorConfig.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentCreateBuilder();
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
