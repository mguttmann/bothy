//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/host.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_updated_payload.g.dart';

/// HostUpdatedPayload
///
/// Properties:
/// * [host] - Never carries `noise_public_key`; clients read keys with `getHost`.
/// * [keyChanged] - True after a re-pairing (24, 3.3).
@BuiltValue()
abstract class HostUpdatedPayload
    implements Built<HostUpdatedPayload, HostUpdatedPayloadBuilder> {
  /// Never carries `noise_public_key`; clients read keys with `getHost`.
  @BuiltValueField(wireName: r'host')
  Host get host;

  /// True after a re-pairing (24, 3.3).
  @BuiltValueField(wireName: r'key_changed')
  bool? get keyChanged;

  HostUpdatedPayload._();

  factory HostUpdatedPayload([void updates(HostUpdatedPayloadBuilder b)]) =
      _$HostUpdatedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostUpdatedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostUpdatedPayload> get serializer =>
      _$HostUpdatedPayloadSerializer();
}

class _$HostUpdatedPayloadSerializer
    implements PrimitiveSerializer<HostUpdatedPayload> {
  @override
  final Iterable<Type> types = const [HostUpdatedPayload, _$HostUpdatedPayload];

  @override
  final String wireName = r'HostUpdatedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostUpdatedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'host';
    yield serializers.serialize(
      object.host,
      specifiedType: const FullType(Host),
    );
    if (object.keyChanged != null) {
      yield r'key_changed';
      yield serializers.serialize(
        object.keyChanged,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    HostUpdatedPayload object, {
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
    required HostUpdatedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'host':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Host),
          ) as Host;
          result.host.replace(valueDes);
          break;
        case r'key_changed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.keyChanged = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostUpdatedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostUpdatedPayloadBuilder();
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
