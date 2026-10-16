//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'quiet_hours_rule.g.dart';

/// QuietHoursRule
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [weekdays] - Bit mask, bit 0 = Monday.
/// * [startLocal] - Local time `HH:MM`.
/// * [endLocal]
/// * [timeZone] - IANA time zone name; absent means the person's time zone (users.time_zone).
/// * [enabled]
/// * [version]
@BuiltValue()
abstract class QuietHoursRule
    implements Built<QuietHoursRule, QuietHoursRuleBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// Bit mask, bit 0 = Monday.
  @BuiltValueField(wireName: r'weekdays')
  int get weekdays;

  /// Local time `HH:MM`.
  @BuiltValueField(wireName: r'start_local')
  String get startLocal;

  @BuiltValueField(wireName: r'end_local')
  String get endLocal;

  /// IANA time zone name; absent means the person's time zone (users.time_zone).
  @BuiltValueField(wireName: r'time_zone')
  String? get timeZone;

  @BuiltValueField(wireName: r'enabled')
  bool get enabled;

  @BuiltValueField(wireName: r'version')
  int get version;

  QuietHoursRule._();

  factory QuietHoursRule([void updates(QuietHoursRuleBuilder b)]) =
      _$QuietHoursRule;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(QuietHoursRuleBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<QuietHoursRule> get serializer =>
      _$QuietHoursRuleSerializer();
}

class _$QuietHoursRuleSerializer
    implements PrimitiveSerializer<QuietHoursRule> {
  @override
  final Iterable<Type> types = const [QuietHoursRule, _$QuietHoursRule];

  @override
  final String wireName = r'QuietHoursRule';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    QuietHoursRule object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
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
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    QuietHoursRule object, {
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
    required QuietHoursRuleBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
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
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  QuietHoursRule deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = QuietHoursRuleBuilder();
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
