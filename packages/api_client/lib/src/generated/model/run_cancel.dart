//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_cancel.g.dart';

/// RunCancel
///
/// Properties:
/// * [reason]
@BuiltValue()
abstract class RunCancel implements Built<RunCancel, RunCancelBuilder> {
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  RunCancel._();

  factory RunCancel([void updates(RunCancelBuilder b)]) = _$RunCancel;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunCancelBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunCancel> get serializer => _$RunCancelSerializer();
}

class _$RunCancelSerializer implements PrimitiveSerializer<RunCancel> {
  @override
  final Iterable<Type> types = const [RunCancel, _$RunCancel];

  @override
  final String wireName = r'RunCancel';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunCancel object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.reason != null) {
      yield r'reason';
      yield serializers.serialize(
        object.reason,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunCancel object, {
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
    required RunCancelBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
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
  RunCancel deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunCancelBuilder();
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
