//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_move.g.dart';

/// ThreadMove
///
/// Properties:
/// * [targetChannelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [reason]
@BuiltValue()
abstract class ThreadMove implements Built<ThreadMove, ThreadMoveBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'target_channel_id')
  String get targetChannelId;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  ThreadMove._();

  factory ThreadMove([void updates(ThreadMoveBuilder b)]) = _$ThreadMove;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadMoveBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadMove> get serializer => _$ThreadMoveSerializer();
}

class _$ThreadMoveSerializer implements PrimitiveSerializer<ThreadMove> {
  @override
  final Iterable<Type> types = const [ThreadMove, _$ThreadMove];

  @override
  final String wireName = r'ThreadMove';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadMove object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'target_channel_id';
    yield serializers.serialize(
      object.targetChannelId,
      specifiedType: const FullType(String),
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
    ThreadMove object, {
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
    required ThreadMoveBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'target_channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.targetChannelId = valueDes;
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
  ThreadMove deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadMoveBuilder();
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
