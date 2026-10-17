//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/quiet_hours_rule.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'quiet_hours_list.g.dart';

/// QuietHoursList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class QuietHoursList
    implements Built<QuietHoursList, QuietHoursListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<QuietHoursRule> get items;

  QuietHoursList._();

  factory QuietHoursList([void updates(QuietHoursListBuilder b)]) =
      _$QuietHoursList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(QuietHoursListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<QuietHoursList> get serializer =>
      _$QuietHoursListSerializer();
}

class _$QuietHoursListSerializer
    implements PrimitiveSerializer<QuietHoursList> {
  @override
  final Iterable<Type> types = const [QuietHoursList, _$QuietHoursList];

  @override
  final String wireName = r'QuietHoursList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    QuietHoursList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(QuietHoursRule)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    QuietHoursList object, {
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
    required QuietHoursListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(QuietHoursRule),
            ]),
          ) as BuiltList<QuietHoursRule>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  QuietHoursList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = QuietHoursListBuilder();
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
