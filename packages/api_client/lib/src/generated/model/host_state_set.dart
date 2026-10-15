//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_state_set.g.dart';

/// HostStateSet
///
/// Properties:
/// * [state]
/// * [reason]
@BuiltValue()
abstract class HostStateSet
    implements Built<HostStateSet, HostStateSetBuilder> {
  @BuiltValueField(wireName: r'state')
  HostStateSetStateEnum get state;
  // enum stateEnum {  active,  disabled,  retired,  };

  @BuiltValueField(wireName: r'reason')
  String get reason;

  HostStateSet._();

  factory HostStateSet([void updates(HostStateSetBuilder b)]) = _$HostStateSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostStateSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostStateSet> get serializer => _$HostStateSetSerializer();
}

class _$HostStateSetSerializer implements PrimitiveSerializer<HostStateSet> {
  @override
  final Iterable<Type> types = const [HostStateSet, _$HostStateSet];

  @override
  final String wireName = r'HostStateSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostStateSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(HostStateSetStateEnum),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostStateSet object, {
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
    required HostStateSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostStateSetStateEnum),
          ) as HostStateSetStateEnum;
          result.state = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostStateSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostStateSetBuilder();
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

class HostStateSetStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const HostStateSetStateEnum active = _$hostStateSetStateEnum_active;
  @BuiltValueEnumConst(wireName: r'disabled')
  static const HostStateSetStateEnum disabled =
      _$hostStateSetStateEnum_disabled;
  @BuiltValueEnumConst(wireName: r'retired')
  static const HostStateSetStateEnum retired = _$hostStateSetStateEnum_retired;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostStateSetStateEnum unknownDefaultOpenApi =
      _$hostStateSetStateEnum_unknownDefaultOpenApi;

  static Serializer<HostStateSetStateEnum> get serializer =>
      _$hostStateSetStateEnumSerializer;

  const HostStateSetStateEnum._(String name) : super(name);

  static BuiltSet<HostStateSetStateEnum> get values =>
      _$hostStateSetStateEnumValues;
  static HostStateSetStateEnum valueOf(String name) =>
      _$hostStateSetStateEnumValueOf(name);
}
