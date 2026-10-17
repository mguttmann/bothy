//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'move_position.g.dart';

/// New position relative to siblings. Exactly one of both fields; none moves to the start.
///
/// Properties:
/// * [afterId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [beforeId] - UUIDv7 (RFC 9562), lowercase canonical form.
@BuiltValue()
abstract class MovePosition
    implements Built<MovePosition, MovePositionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'after_id')
  String? get afterId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'before_id')
  String? get beforeId;

  MovePosition._();

  factory MovePosition([void updates(MovePositionBuilder b)]) = _$MovePosition;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MovePositionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<MovePosition> get serializer => _$MovePositionSerializer();
}

class _$MovePositionSerializer implements PrimitiveSerializer<MovePosition> {
  @override
  final Iterable<Type> types = const [MovePosition, _$MovePosition];

  @override
  final String wireName = r'MovePosition';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    MovePosition object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.afterId != null) {
      yield r'after_id';
      yield serializers.serialize(
        object.afterId,
        specifiedType: const FullType(String),
      );
    }
    if (object.beforeId != null) {
      yield r'before_id';
      yield serializers.serialize(
        object.beforeId,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    MovePosition object, {
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
    required MovePositionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'after_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.afterId = valueDes;
          break;
        case r'before_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.beforeId = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  MovePosition deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MovePositionBuilder();
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
