//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_profile.g.dart';

/// Profile reported by the host service (`host_profiles.v1`). Never a command line, path or environment.
///
/// Properties:
/// * [name]
/// * [displayName]
/// * [kind]
@BuiltValue()
abstract class HostProfile implements Built<HostProfile, HostProfileBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'display_name')
  String get displayName;

  @BuiltValueField(wireName: r'kind')
  HostProfileKindEnum get kind;
  // enum kindEnum {  shell,  claude,  command,  };

  HostProfile._();

  factory HostProfile([void updates(HostProfileBuilder b)]) = _$HostProfile;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostProfileBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostProfile> get serializer => _$HostProfileSerializer();
}

class _$HostProfileSerializer implements PrimitiveSerializer<HostProfile> {
  @override
  final Iterable<Type> types = const [HostProfile, _$HostProfile];

  @override
  final String wireName = r'HostProfile';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostProfile object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'display_name';
    yield serializers.serialize(
      object.displayName,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(HostProfileKindEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostProfile object, {
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
    required HostProfileBuilder result,
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
        case r'display_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.displayName = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostProfileKindEnum),
          ) as HostProfileKindEnum;
          result.kind = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostProfile deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostProfileBuilder();
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

class HostProfileKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'shell')
  static const HostProfileKindEnum shell = _$hostProfileKindEnum_shell;
  @BuiltValueEnumConst(wireName: r'claude')
  static const HostProfileKindEnum claude = _$hostProfileKindEnum_claude;
  @BuiltValueEnumConst(wireName: r'command')
  static const HostProfileKindEnum command = _$hostProfileKindEnum_command;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostProfileKindEnum unknownDefaultOpenApi =
      _$hostProfileKindEnum_unknownDefaultOpenApi;

  static Serializer<HostProfileKindEnum> get serializer =>
      _$hostProfileKindEnumSerializer;

  const HostProfileKindEnum._(String name) : super(name);

  static BuiltSet<HostProfileKindEnum> get values =>
      _$hostProfileKindEnumValues;
  static HostProfileKindEnum valueOf(String name) =>
      _$hostProfileKindEnumValueOf(name);
}
