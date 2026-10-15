//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_resume.g.dart';

/// GatewayResume
///
/// Properties:
/// * [gatewaySessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [accessToken]
/// * [apiToken]
/// * [dpop]
/// * [cursor]
/// * [syncEpoch] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class GatewayResume
    implements Built<GatewayResume, GatewayResumeBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'gateway_session_id')
  String get gatewaySessionId;

  @BuiltValueField(wireName: r'access_token')
  String? get accessToken;

  @BuiltValueField(wireName: r'api_token')
  String? get apiToken;

  @BuiltValueField(wireName: r'dpop')
  String? get dpop;

  @BuiltValueField(wireName: r'cursor')
  int get cursor;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'sync_epoch')
  String? get syncEpoch;

  GatewayResume._();

  factory GatewayResume([void updates(GatewayResumeBuilder b)]) =
      _$GatewayResume;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayResumeBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayResume> get serializer =>
      _$GatewayResumeSerializer();
}

class _$GatewayResumeSerializer implements PrimitiveSerializer<GatewayResume> {
  @override
  final Iterable<Type> types = const [GatewayResume, _$GatewayResume];

  @override
  final String wireName = r'GatewayResume';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayResume object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'gateway_session_id';
    yield serializers.serialize(
      object.gatewaySessionId,
      specifiedType: const FullType(String),
    );
    if (object.accessToken != null) {
      yield r'access_token';
      yield serializers.serialize(
        object.accessToken,
        specifiedType: const FullType(String),
      );
    }
    if (object.apiToken != null) {
      yield r'api_token';
      yield serializers.serialize(
        object.apiToken,
        specifiedType: const FullType(String),
      );
    }
    if (object.dpop != null) {
      yield r'dpop';
      yield serializers.serialize(
        object.dpop,
        specifiedType: const FullType(String),
      );
    }
    yield r'cursor';
    yield serializers.serialize(
      object.cursor,
      specifiedType: const FullType(int),
    );
    if (object.syncEpoch != null) {
      yield r'sync_epoch';
      yield serializers.serialize(
        object.syncEpoch,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayResume object, {
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
    required GatewayResumeBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'gateway_session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.gatewaySessionId = valueDes;
          break;
        case r'access_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.accessToken = valueDes;
          break;
        case r'api_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.apiToken = valueDes;
          break;
        case r'dpop':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.dpop = valueDes;
          break;
        case r'cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.cursor = valueDes;
          break;
        case r'sync_epoch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.syncEpoch = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayResume deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayResumeBuilder();
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
