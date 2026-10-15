//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'evaluated_flag.g.dart';

/// EvaluatedFlag
///
/// Properties:
/// * [key]
/// * [enabled]
@BuiltValue()
abstract class EvaluatedFlag
    implements Built<EvaluatedFlag, EvaluatedFlagBuilder> {
  @BuiltValueField(wireName: r'key')
  String get key;

  @BuiltValueField(wireName: r'enabled')
  bool get enabled;

  EvaluatedFlag._();

  factory EvaluatedFlag([void updates(EvaluatedFlagBuilder b)]) =
      _$EvaluatedFlag;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(EvaluatedFlagBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<EvaluatedFlag> get serializer =>
      _$EvaluatedFlagSerializer();
}

class _$EvaluatedFlagSerializer implements PrimitiveSerializer<EvaluatedFlag> {
  @override
  final Iterable<Type> types = const [EvaluatedFlag, _$EvaluatedFlag];

  @override
  final String wireName = r'EvaluatedFlag';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    EvaluatedFlag object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'key';
    yield serializers.serialize(
      object.key,
      specifiedType: const FullType(String),
    );
    yield r'enabled';
    yield serializers.serialize(
      object.enabled,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    EvaluatedFlag object, {
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
    required EvaluatedFlagBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.key = valueDes;
          break;
        case r'enabled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.enabled = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  EvaluatedFlag deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = EvaluatedFlagBuilder();
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
