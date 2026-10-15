//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/quiet_hours_rule.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'quiet_hours_payload.g.dart';

/// QuietHoursPayload
///
/// Properties:
/// * [rules] - Complete list after the change.
@BuiltValue()
abstract class QuietHoursPayload
    implements Built<QuietHoursPayload, QuietHoursPayloadBuilder> {
  /// Complete list after the change.
  @BuiltValueField(wireName: r'rules')
  BuiltList<QuietHoursRule> get rules;

  QuietHoursPayload._();

  factory QuietHoursPayload([void updates(QuietHoursPayloadBuilder b)]) =
      _$QuietHoursPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(QuietHoursPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<QuietHoursPayload> get serializer =>
      _$QuietHoursPayloadSerializer();
}

class _$QuietHoursPayloadSerializer
    implements PrimitiveSerializer<QuietHoursPayload> {
  @override
  final Iterable<Type> types = const [QuietHoursPayload, _$QuietHoursPayload];

  @override
  final String wireName = r'QuietHoursPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    QuietHoursPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'rules';
    yield serializers.serialize(
      object.rules,
      specifiedType: const FullType(BuiltList, [FullType(QuietHoursRule)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    QuietHoursPayload object, {
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
    required QuietHoursPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'rules':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(QuietHoursRule),
            ]),
          ) as BuiltList<QuietHoursRule>;
          result.rules.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  QuietHoursPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = QuietHoursPayloadBuilder();
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
