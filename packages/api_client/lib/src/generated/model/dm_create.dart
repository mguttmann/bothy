//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'dm_create.g.dart';

/// DmCreate
///
/// Properties:
/// * [participantIds] - Other people in the conversation; the caller is added.
@BuiltValue()
abstract class DmCreate implements Built<DmCreate, DmCreateBuilder> {
  /// Other people in the conversation; the caller is added.
  @BuiltValueField(wireName: r'participant_ids')
  BuiltList<String> get participantIds;

  DmCreate._();

  factory DmCreate([void updates(DmCreateBuilder b)]) = _$DmCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DmCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DmCreate> get serializer => _$DmCreateSerializer();
}

class _$DmCreateSerializer implements PrimitiveSerializer<DmCreate> {
  @override
  final Iterable<Type> types = const [DmCreate, _$DmCreate];

  @override
  final String wireName = r'DmCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DmCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'participant_ids';
    yield serializers.serialize(
      object.participantIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    DmCreate object, {
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
    required DmCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'participant_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.participantIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DmCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DmCreateBuilder();
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
