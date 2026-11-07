//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_cancel.g.dart';

/// RunWakeCancel
///
/// Properties:
/// * [reason] - Reason code of a cancel by the agent, for example `usage_cleared`; a person's cancel has `cancelled_by_person`.
@BuiltValue()
abstract class RunWakeCancel
    implements Built<RunWakeCancel, RunWakeCancelBuilder> {
  /// Reason code of a cancel by the agent, for example `usage_cleared`; a person's cancel has `cancelled_by_person`.
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  RunWakeCancel._();

  factory RunWakeCancel([void updates(RunWakeCancelBuilder b)]) =
      _$RunWakeCancel;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakeCancelBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakeCancel> get serializer =>
      _$RunWakeCancelSerializer();
}

class _$RunWakeCancelSerializer implements PrimitiveSerializer<RunWakeCancel> {
  @override
  final Iterable<Type> types = const [RunWakeCancel, _$RunWakeCancel];

  @override
  final String wireName = r'RunWakeCancel';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakeCancel object, {
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
    RunWakeCancel object, {
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
    required RunWakeCancelBuilder result,
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
  RunWakeCancel deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakeCancelBuilder();
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
