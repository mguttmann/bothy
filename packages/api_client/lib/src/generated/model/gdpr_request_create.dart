//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/gdpr_request_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gdpr_request_create.g.dart';

/// GdprRequestCreate
///
/// Properties:
/// * [subjectUserId] - Absent means the caller. Others only for owners.
/// * [kind]
/// * [reason]
@BuiltValue()
abstract class GdprRequestCreate
    implements Built<GdprRequestCreate, GdprRequestCreateBuilder> {
  /// Absent means the caller. Others only for owners.
  @BuiltValueField(wireName: r'subject_user_id')
  String? get subjectUserId;

  @BuiltValueField(wireName: r'kind')
  GdprRequestKind get kind;
  // enum kindEnum {  export,  deletion,  rectification,  };

  @BuiltValueField(wireName: r'reason')
  String get reason;

  GdprRequestCreate._();

  factory GdprRequestCreate([void updates(GdprRequestCreateBuilder b)]) =
      _$GdprRequestCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GdprRequestCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GdprRequestCreate> get serializer =>
      _$GdprRequestCreateSerializer();
}

class _$GdprRequestCreateSerializer
    implements PrimitiveSerializer<GdprRequestCreate> {
  @override
  final Iterable<Type> types = const [GdprRequestCreate, _$GdprRequestCreate];

  @override
  final String wireName = r'GdprRequestCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GdprRequestCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.subjectUserId != null) {
      yield r'subject_user_id';
      yield serializers.serialize(
        object.subjectUserId,
        specifiedType: const FullType(String),
      );
    }
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(GdprRequestKind),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GdprRequestCreate object, {
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
    required GdprRequestCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'subject_user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.subjectUserId = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GdprRequestKind),
          ) as GdprRequestKind;
          result.kind = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
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
  GdprRequestCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GdprRequestCreateBuilder();
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
