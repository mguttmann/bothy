//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'quiet_hours_rule_input.g.dart';

/// QuietHoursRuleInput
///
/// Properties:
/// * [weekdays]
/// * [startLocal]
/// * [endLocal]
/// * [timeZone]
/// * [enabled]
@BuiltValue()
abstract class QuietHoursRuleInput
    implements Built<QuietHoursRuleInput, QuietHoursRuleInputBuilder> {
  @BuiltValueField(wireName: r'weekdays')
  int get weekdays;

  @BuiltValueField(wireName: r'start_local')
  String get startLocal;

  @BuiltValueField(wireName: r'end_local')
  String get endLocal;

  @BuiltValueField(wireName: r'time_zone')
  String? get timeZone;

  @BuiltValueField(wireName: r'enabled')
  bool get enabled;

  QuietHoursRuleInput._();

  factory QuietHoursRuleInput([void updates(QuietHoursRuleInputBuilder b)]) =
      _$QuietHoursRuleInput;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(QuietHoursRuleInputBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<QuietHoursRuleInput> get serializer =>
      _$QuietHoursRuleInputSerializer();
}

class _$QuietHoursRuleInputSerializer
    implements PrimitiveSerializer<QuietHoursRuleInput> {
  @override
  final Iterable<Type> types = const [
    QuietHoursRuleInput,
    _$QuietHoursRuleInput,
  ];

  @override
  final String wireName = r'QuietHoursRuleInput';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    QuietHoursRuleInput object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'weekdays';
    yield serializers.serialize(
      object.weekdays,
      specifiedType: const FullType(int),
    );
    yield r'start_local';
    yield serializers.serialize(
      object.startLocal,
      specifiedType: const FullType(String),
    );
    yield r'end_local';
    yield serializers.serialize(
      object.endLocal,
      specifiedType: const FullType(String),
    );
    if (object.timeZone != null) {
      yield r'time_zone';
      yield serializers.serialize(
        object.timeZone,
        specifiedType: const FullType(String),
      );
    }
    yield r'enabled';
    yield serializers.serialize(
      object.enabled,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    QuietHoursRuleInput object, {
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
    required QuietHoursRuleInputBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'weekdays':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.weekdays = valueDes;
          break;
        case r'start_local':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.startLocal = valueDes;
          break;
        case r'end_local':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.endLocal = valueDes;
          break;
        case r'time_zone':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.timeZone = valueDes;
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
  QuietHoursRuleInput deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = QuietHoursRuleInputBuilder();
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
