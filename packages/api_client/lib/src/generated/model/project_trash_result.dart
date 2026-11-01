//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_trash_result.g.dart';

/// ProjectTrashResult
///
/// Properties:
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [result]
/// * [reason] - The refusal code with `result` `refused` (09 18.15).
@BuiltValue()
abstract class ProjectTrashResult
    implements Built<ProjectTrashResult, ProjectTrashResultBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  @BuiltValueField(wireName: r'result')
  ProjectTrashResultResultEnum get result;
  // enum resultEnum {  trashed,  refused,  };

  /// The refusal code with `result` `refused` (09 18.15).
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  ProjectTrashResult._();

  factory ProjectTrashResult([void updates(ProjectTrashResultBuilder b)]) =
      _$ProjectTrashResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectTrashResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectTrashResult> get serializer =>
      _$ProjectTrashResultSerializer();
}

class _$ProjectTrashResultSerializer
    implements PrimitiveSerializer<ProjectTrashResult> {
  @override
  final Iterable<Type> types = const [ProjectTrashResult, _$ProjectTrashResult];

  @override
  final String wireName = r'ProjectTrashResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectTrashResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'result';
    yield serializers.serialize(
      object.result,
      specifiedType: const FullType(ProjectTrashResultResultEnum),
    );
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
    ProjectTrashResult object, {
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
    required ProjectTrashResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'result':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ProjectTrashResultResultEnum),
          ) as ProjectTrashResultResultEnum;
          result.result = valueDes;
          break;
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
  ProjectTrashResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectTrashResultBuilder();
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

class ProjectTrashResultResultEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'trashed')
  static const ProjectTrashResultResultEnum trashed =
      _$projectTrashResultResultEnum_trashed;
  @BuiltValueEnumConst(wireName: r'refused')
  static const ProjectTrashResultResultEnum refused =
      _$projectTrashResultResultEnum_refused;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ProjectTrashResultResultEnum unknownDefaultOpenApi =
      _$projectTrashResultResultEnum_unknownDefaultOpenApi;

  static Serializer<ProjectTrashResultResultEnum> get serializer =>
      _$projectTrashResultResultEnumSerializer;

  const ProjectTrashResultResultEnum._(String name) : super(name);

  static BuiltSet<ProjectTrashResultResultEnum> get values =>
      _$projectTrashResultResultEnumValues;
  static ProjectTrashResultResultEnum valueOf(String name) =>
      _$projectTrashResultResultEnumValueOf(name);
}
