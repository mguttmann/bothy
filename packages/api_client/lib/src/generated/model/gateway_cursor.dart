//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_cursor.g.dart';

/// GatewayCursor
///
/// Properties:
/// * [seq]
@BuiltValue()
abstract class GatewayCursor
    implements Built<GatewayCursor, GatewayCursorBuilder> {
  @BuiltValueField(wireName: r'seq')
  int get seq;

  GatewayCursor._();

  factory GatewayCursor([void updates(GatewayCursorBuilder b)]) =
      _$GatewayCursor;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayCursorBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayCursor> get serializer =>
      _$GatewayCursorSerializer();
}

class _$GatewayCursorSerializer implements PrimitiveSerializer<GatewayCursor> {
  @override
  final Iterable<Type> types = const [GatewayCursor, _$GatewayCursor];

  @override
  final String wireName = r'GatewayCursor';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayCursor object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'seq';
    yield serializers.serialize(object.seq, specifiedType: const FullType(int));
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayCursor object, {
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
    required GatewayCursorBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.seq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayCursor deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayCursorBuilder();
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
