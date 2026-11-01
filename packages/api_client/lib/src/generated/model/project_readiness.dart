//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/project_trash_check_report.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_readiness.g.dart';

/// ProjectReadiness
///
/// Properties:
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [ready]
/// * [reason] - Why the forum is not ready, a code of 09 18.12 (for example `provisioning`, `workspace_exists`, `clone_failed`); only with `ready` false. Stored as `project_bindings.ready_reason`.
/// * [trashCheck] - Runners only, the folder check of 09 18.15, at most every `runner.trash_check_interval`; stored on the binding. Absent means unchanged.
@BuiltValue()
abstract class ProjectReadiness
    implements Built<ProjectReadiness, ProjectReadinessBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  @BuiltValueField(wireName: r'ready')
  bool get ready;

  /// Why the forum is not ready, a code of 09 18.12 (for example `provisioning`, `workspace_exists`, `clone_failed`); only with `ready` false. Stored as `project_bindings.ready_reason`.
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  /// Runners only, the folder check of 09 18.15, at most every `runner.trash_check_interval`; stored on the binding. Absent means unchanged.
  @BuiltValueField(wireName: r'trash_check')
  ProjectTrashCheckReport? get trashCheck;

  ProjectReadiness._();

  factory ProjectReadiness([void updates(ProjectReadinessBuilder b)]) =
      _$ProjectReadiness;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectReadinessBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectReadiness> get serializer =>
      _$ProjectReadinessSerializer();
}

class _$ProjectReadinessSerializer
    implements PrimitiveSerializer<ProjectReadiness> {
  @override
  final Iterable<Type> types = const [ProjectReadiness, _$ProjectReadiness];

  @override
  final String wireName = r'ProjectReadiness';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectReadiness object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'ready';
    yield serializers.serialize(
      object.ready,
      specifiedType: const FullType(bool),
    );
    if (object.reason != null) {
      yield r'reason';
      yield serializers.serialize(
        object.reason,
        specifiedType: const FullType(String),
      );
    }
    if (object.trashCheck != null) {
      yield r'trash_check';
      yield serializers.serialize(
        object.trashCheck,
        specifiedType: const FullType(ProjectTrashCheckReport),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectReadiness object, {
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
    required ProjectReadinessBuilder result,
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
        case r'ready':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.ready = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.reason = valueDes;
          break;
        case r'trash_check':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ProjectTrashCheckReport),
          ) as ProjectTrashCheckReport?;
          if (valueDes == null) continue;
          result.trashCheck.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectReadiness deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectReadinessBuilder();
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
