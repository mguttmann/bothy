//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_slots.g.dart';

/// Slot use of a runner (09 4.8, 18.2), `used` turns run of at most `total` (`runner.max_parallel_runs`) at once; `used` is at most `total`.
///
/// Properties:
/// * [used]
/// * [total]
@BuiltValue()
abstract class RunSlots implements Built<RunSlots, RunSlotsBuilder> {
  @BuiltValueField(wireName: r'used')
  int get used;

  @BuiltValueField(wireName: r'total')
  int get total;

  RunSlots._();

  factory RunSlots([void updates(RunSlotsBuilder b)]) = _$RunSlots;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunSlotsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunSlots> get serializer => _$RunSlotsSerializer();
}

class _$RunSlotsSerializer implements PrimitiveSerializer<RunSlots> {
  @override
  final Iterable<Type> types = const [RunSlots, _$RunSlots];

  @override
  final String wireName = r'RunSlots';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunSlots object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'used';
    yield serializers.serialize(
      object.used,
      specifiedType: const FullType(int),
    );
    yield r'total';
    yield serializers.serialize(
      object.total,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunSlots object, {
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
    required RunSlotsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'used':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.used = valueDes;
          break;
        case r'total':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.total = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunSlots deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunSlotsBuilder();
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
