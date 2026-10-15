//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_pair_result.g.dart';

/// HostPairResult
///
/// Properties:
/// * [hostId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [keyFingerprint]
/// * [instanceId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class HostPairResult
    implements Built<HostPairResult, HostPairResultBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'host_id')
  String get hostId;

  @BuiltValueField(wireName: r'key_fingerprint')
  String get keyFingerprint;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'instance_id')
  String get instanceId;

  HostPairResult._();

  factory HostPairResult([void updates(HostPairResultBuilder b)]) =
      _$HostPairResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostPairResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostPairResult> get serializer =>
      _$HostPairResultSerializer();
}

class _$HostPairResultSerializer
    implements PrimitiveSerializer<HostPairResult> {
  @override
  final Iterable<Type> types = const [HostPairResult, _$HostPairResult];

  @override
  final String wireName = r'HostPairResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostPairResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'host_id';
    yield serializers.serialize(
      object.hostId,
      specifiedType: const FullType(String),
    );
    yield r'key_fingerprint';
    yield serializers.serialize(
      object.keyFingerprint,
      specifiedType: const FullType(String),
    );
    yield r'instance_id';
    yield serializers.serialize(
      object.instanceId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostPairResult object, {
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
    required HostPairResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'host_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.hostId = valueDes;
          break;
        case r'key_fingerprint':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.keyFingerprint = valueDes;
          break;
        case r'instance_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.instanceId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostPairResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostPairResultBuilder();
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
