//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'instance_updated_payload.g.dart';

/// InstanceUpdatedPayload
///
/// Properties:
/// * [changedKeys] - Setting or flag keys that changed; values are loaded via REST.
@BuiltValue()
abstract class InstanceUpdatedPayload
    implements Built<InstanceUpdatedPayload, InstanceUpdatedPayloadBuilder> {
  /// Setting or flag keys that changed; values are loaded via REST.
  @BuiltValueField(wireName: r'changed_keys')
  BuiltList<String> get changedKeys;

  InstanceUpdatedPayload._();

  factory InstanceUpdatedPayload([
    void updates(InstanceUpdatedPayloadBuilder b),
  ]) = _$InstanceUpdatedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InstanceUpdatedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InstanceUpdatedPayload> get serializer =>
      _$InstanceUpdatedPayloadSerializer();
}

class _$InstanceUpdatedPayloadSerializer
    implements PrimitiveSerializer<InstanceUpdatedPayload> {
  @override
  final Iterable<Type> types = const [
    InstanceUpdatedPayload,
    _$InstanceUpdatedPayload,
  ];

  @override
  final String wireName = r'InstanceUpdatedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InstanceUpdatedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'changed_keys';
    yield serializers.serialize(
      object.changedKeys,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    InstanceUpdatedPayload object, {
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
    required InstanceUpdatedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'changed_keys':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.changedKeys.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InstanceUpdatedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InstanceUpdatedPayloadBuilder();
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
