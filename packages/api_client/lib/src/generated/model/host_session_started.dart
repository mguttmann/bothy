//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_session_started.g.dart';

/// HostSessionStarted
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class HostSessionStarted
    implements Built<HostSessionStarted, HostSessionStartedBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  HostSessionStarted._();

  factory HostSessionStarted([void updates(HostSessionStartedBuilder b)]) =
      _$HostSessionStarted;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostSessionStartedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostSessionStarted> get serializer =>
      _$HostSessionStartedSerializer();
}

class _$HostSessionStartedSerializer
    implements PrimitiveSerializer<HostSessionStarted> {
  @override
  final Iterable<Type> types = const [HostSessionStarted, _$HostSessionStarted];

  @override
  final String wireName = r'HostSessionStarted';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostSessionStarted object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostSessionStarted object, {
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
    required HostSessionStartedBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sessionId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostSessionStarted deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostSessionStartedBuilder();
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
