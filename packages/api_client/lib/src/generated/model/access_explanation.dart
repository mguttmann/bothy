//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/access_target_kind.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/access_entry.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_explanation.g.dart';

/// AccessExplanation
///
/// Properties:
/// * [targetKind]
/// * [targetId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [entries]
@BuiltValue()
abstract class AccessExplanation
    implements Built<AccessExplanation, AccessExplanationBuilder> {
  @BuiltValueField(wireName: r'target_kind')
  AccessTargetKind get targetKind;
  // enum targetKindEnum {  space,  channel,  host,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'target_id')
  String get targetId;

  @BuiltValueField(wireName: r'entries')
  BuiltList<AccessEntry> get entries;

  AccessExplanation._();

  factory AccessExplanation([void updates(AccessExplanationBuilder b)]) =
      _$AccessExplanation;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AccessExplanationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AccessExplanation> get serializer =>
      _$AccessExplanationSerializer();
}

class _$AccessExplanationSerializer
    implements PrimitiveSerializer<AccessExplanation> {
  @override
  final Iterable<Type> types = const [AccessExplanation, _$AccessExplanation];

  @override
  final String wireName = r'AccessExplanation';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AccessExplanation object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'target_kind';
    yield serializers.serialize(
      object.targetKind,
      specifiedType: const FullType(AccessTargetKind),
    );
    yield r'target_id';
    yield serializers.serialize(
      object.targetId,
      specifiedType: const FullType(String),
    );
    yield r'entries';
    yield serializers.serialize(
      object.entries,
      specifiedType: const FullType(BuiltList, [FullType(AccessEntry)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AccessExplanation object, {
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
    required AccessExplanationBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AccessTargetKind),
          ) as AccessTargetKind;
          result.targetKind = valueDes;
          break;
        case r'target_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.targetId = valueDes;
          break;
        case r'entries':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AccessEntry)]),
          ) as BuiltList<AccessEntry>;
          result.entries.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AccessExplanation deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AccessExplanationBuilder();
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
