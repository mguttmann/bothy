//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/project_trash_check.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_folder_deletion.g.dart';

/// The project folder of a forum with a project binding; only for owners (`channel.project.bind`).
///
/// Properties:
/// * [workspaceRef]
/// * [trashCheck]
/// * [trashState] - Absent when no move was requested; `requested` until the runner reports.
/// * [trashReason] - The refusal code of the runner with `trash_state` `refused` (09 18.15).
@BuiltValue()
abstract class ProjectFolderDeletion
    implements Built<ProjectFolderDeletion, ProjectFolderDeletionBuilder> {
  @BuiltValueField(wireName: r'workspace_ref')
  String get workspaceRef;

  @BuiltValueField(wireName: r'trash_check')
  ProjectTrashCheck? get trashCheck;

  /// Absent when no move was requested; `requested` until the runner reports.
  @BuiltValueField(wireName: r'trash_state')
  ProjectFolderDeletionTrashStateEnum? get trashState;
  // enum trashStateEnum {  requested,  trashed,  refused,  };

  /// The refusal code of the runner with `trash_state` `refused` (09 18.15).
  @BuiltValueField(wireName: r'trash_reason')
  String? get trashReason;

  ProjectFolderDeletion._();

  factory ProjectFolderDeletion([
    void updates(ProjectFolderDeletionBuilder b),
  ]) = _$ProjectFolderDeletion;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectFolderDeletionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectFolderDeletion> get serializer =>
      _$ProjectFolderDeletionSerializer();
}

class _$ProjectFolderDeletionSerializer
    implements PrimitiveSerializer<ProjectFolderDeletion> {
  @override
  final Iterable<Type> types = const [
    ProjectFolderDeletion,
    _$ProjectFolderDeletion,
  ];

  @override
  final String wireName = r'ProjectFolderDeletion';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectFolderDeletion object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'workspace_ref';
    yield serializers.serialize(
      object.workspaceRef,
      specifiedType: const FullType(String),
    );
    if (object.trashCheck != null) {
      yield r'trash_check';
      yield serializers.serialize(
        object.trashCheck,
        specifiedType: const FullType(ProjectTrashCheck),
      );
    }
    if (object.trashState != null) {
      yield r'trash_state';
      yield serializers.serialize(
        object.trashState,
        specifiedType: const FullType(ProjectFolderDeletionTrashStateEnum),
      );
    }
    if (object.trashReason != null) {
      yield r'trash_reason';
      yield serializers.serialize(
        object.trashReason,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectFolderDeletion object, {
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
    required ProjectFolderDeletionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'workspace_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.workspaceRef = valueDes;
          break;
        case r'trash_check':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ProjectTrashCheck),
          ) as ProjectTrashCheck?;
          if (valueDes == null) continue;
          result.trashCheck.replace(valueDes);
          break;
        case r'trash_state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              ProjectFolderDeletionTrashStateEnum,
            ),
          ) as ProjectFolderDeletionTrashStateEnum?;
          if (valueDes == null) continue;
          result.trashState = valueDes;
          break;
        case r'trash_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.trashReason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectFolderDeletion deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectFolderDeletionBuilder();
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

/// Absent when no move was requested; `requested` until the runner reports.
class ProjectFolderDeletionTrashStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'requested')
  static const ProjectFolderDeletionTrashStateEnum requested =
      _$projectFolderDeletionTrashStateEnum_requested;
  @BuiltValueEnumConst(wireName: r'trashed')
  static const ProjectFolderDeletionTrashStateEnum trashed =
      _$projectFolderDeletionTrashStateEnum_trashed;
  @BuiltValueEnumConst(wireName: r'refused')
  static const ProjectFolderDeletionTrashStateEnum refused =
      _$projectFolderDeletionTrashStateEnum_refused;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ProjectFolderDeletionTrashStateEnum unknownDefaultOpenApi =
      _$projectFolderDeletionTrashStateEnum_unknownDefaultOpenApi;

  static Serializer<ProjectFolderDeletionTrashStateEnum> get serializer =>
      _$projectFolderDeletionTrashStateEnumSerializer;

  const ProjectFolderDeletionTrashStateEnum._(String name) : super(name);

  static BuiltSet<ProjectFolderDeletionTrashStateEnum> get values =>
      _$projectFolderDeletionTrashStateEnumValues;
  static ProjectFolderDeletionTrashStateEnum valueOf(String name) =>
      _$projectFolderDeletionTrashStateEnumValueOf(name);
}
