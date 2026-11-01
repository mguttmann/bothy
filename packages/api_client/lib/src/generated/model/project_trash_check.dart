//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_trash_check.g.dart';

/// Whether the serving runner would move the folder to the trash now (09 18.15). Absent in `ProjectFolderDeletion` before the runner's first check.
///
/// Properties:
/// * [ok]
/// * [reason] - Why not, with `ok` false; a code of 09 18.15 (`outside_root`, `symlink`, `foreign_worktree`, `uncommitted_changes`, `unpushed_commits`, `contains_other_project`, `workspace_missing`, `check_failed`).
/// * [checkedAt] - When the server received the check.
@BuiltValue()
abstract class ProjectTrashCheck
    implements Built<ProjectTrashCheck, ProjectTrashCheckBuilder> {
  @BuiltValueField(wireName: r'ok')
  bool get ok;

  /// Why not, with `ok` false; a code of 09 18.15 (`outside_root`, `symlink`, `foreign_worktree`, `uncommitted_changes`, `unpushed_commits`, `contains_other_project`, `workspace_missing`, `check_failed`).
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  /// When the server received the check.
  @BuiltValueField(wireName: r'checked_at')
  DateTime? get checkedAt;

  ProjectTrashCheck._();

  factory ProjectTrashCheck([void updates(ProjectTrashCheckBuilder b)]) =
      _$ProjectTrashCheck;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectTrashCheckBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectTrashCheck> get serializer =>
      _$ProjectTrashCheckSerializer();
}

class _$ProjectTrashCheckSerializer
    implements PrimitiveSerializer<ProjectTrashCheck> {
  @override
  final Iterable<Type> types = const [ProjectTrashCheck, _$ProjectTrashCheck];

  @override
  final String wireName = r'ProjectTrashCheck';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectTrashCheck object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'ok';
    yield serializers.serialize(object.ok, specifiedType: const FullType(bool));
    if (object.reason != null) {
      yield r'reason';
      yield serializers.serialize(
        object.reason,
        specifiedType: const FullType(String),
      );
    }
    if (object.checkedAt != null) {
      yield r'checked_at';
      yield serializers.serialize(
        object.checkedAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectTrashCheck object, {
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
    required ProjectTrashCheckBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'ok':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.ok = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.reason = valueDes;
          break;
        case r'checked_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.checkedAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectTrashCheck deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectTrashCheckBuilder();
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
