//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/project_folder_deletion.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/deletion_contents.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'deletion_info.g.dart';

/// DeletionInfo
///
/// Properties:
/// * [targetKind]
/// * [targetId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state] - `live` before a deletion (a preview), `pending` while it can be undone, `final` after `undo_until`.
/// * [deletedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [undoUntil] - The end of the undo window, `deleted_at` plus `chat.delete_undo_window`; absent while `live`.
/// * [contents]
/// * [projectFolder]
@BuiltValue()
abstract class DeletionInfo
    implements Built<DeletionInfo, DeletionInfoBuilder> {
  @BuiltValueField(wireName: r'target_kind')
  DeletionInfoTargetKindEnum get targetKind;
  // enum targetKindEnum {  space,  channel,  thread,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'target_id')
  String get targetId;

  /// `live` before a deletion (a preview), `pending` while it can be undone, `final` after `undo_until`.
  @BuiltValueField(wireName: r'state')
  DeletionInfoStateEnum get state;
  // enum stateEnum {  live,  pending,  final,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'deleted_at')
  DateTime? get deletedAt;

  /// The end of the undo window, `deleted_at` plus `chat.delete_undo_window`; absent while `live`.
  @BuiltValueField(wireName: r'undo_until')
  DateTime? get undoUntil;

  @BuiltValueField(wireName: r'contents')
  DeletionContents get contents;

  @BuiltValueField(wireName: r'project_folder')
  ProjectFolderDeletion? get projectFolder;

  DeletionInfo._();

  factory DeletionInfo([void updates(DeletionInfoBuilder b)]) = _$DeletionInfo;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DeletionInfoBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DeletionInfo> get serializer => _$DeletionInfoSerializer();
}

class _$DeletionInfoSerializer implements PrimitiveSerializer<DeletionInfo> {
  @override
  final Iterable<Type> types = const [DeletionInfo, _$DeletionInfo];

  @override
  final String wireName = r'DeletionInfo';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DeletionInfo object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'target_kind';
    yield serializers.serialize(
      object.targetKind,
      specifiedType: const FullType(DeletionInfoTargetKindEnum),
    );
    yield r'target_id';
    yield serializers.serialize(
      object.targetId,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(DeletionInfoStateEnum),
    );
    if (object.deletedAt != null) {
      yield r'deleted_at';
      yield serializers.serialize(
        object.deletedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.undoUntil != null) {
      yield r'undo_until';
      yield serializers.serialize(
        object.undoUntil,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'contents';
    yield serializers.serialize(
      object.contents,
      specifiedType: const FullType(DeletionContents),
    );
    if (object.projectFolder != null) {
      yield r'project_folder';
      yield serializers.serialize(
        object.projectFolder,
        specifiedType: const FullType(ProjectFolderDeletion),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    DeletionInfo object, {
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
    required DeletionInfoBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DeletionInfoTargetKindEnum),
          ) as DeletionInfoTargetKindEnum;
          result.targetKind = valueDes;
          break;
        case r'target_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.targetId = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DeletionInfoStateEnum),
          ) as DeletionInfoStateEnum;
          result.state = valueDes;
          break;
        case r'deleted_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.deletedAt = valueDes;
          break;
        case r'undo_until':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.undoUntil = valueDes;
          break;
        case r'contents':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DeletionContents),
          ) as DeletionContents;
          result.contents.replace(valueDes);
          break;
        case r'project_folder':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ProjectFolderDeletion),
          ) as ProjectFolderDeletion?;
          if (valueDes == null) continue;
          result.projectFolder.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DeletionInfo deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DeletionInfoBuilder();
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

class DeletionInfoTargetKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'space')
  static const DeletionInfoTargetKindEnum space =
      _$deletionInfoTargetKindEnum_space;
  @BuiltValueEnumConst(wireName: r'channel')
  static const DeletionInfoTargetKindEnum channel =
      _$deletionInfoTargetKindEnum_channel;
  @BuiltValueEnumConst(wireName: r'thread')
  static const DeletionInfoTargetKindEnum thread =
      _$deletionInfoTargetKindEnum_thread;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const DeletionInfoTargetKindEnum unknownDefaultOpenApi =
      _$deletionInfoTargetKindEnum_unknownDefaultOpenApi;

  static Serializer<DeletionInfoTargetKindEnum> get serializer =>
      _$deletionInfoTargetKindEnumSerializer;

  const DeletionInfoTargetKindEnum._(String name) : super(name);

  static BuiltSet<DeletionInfoTargetKindEnum> get values =>
      _$deletionInfoTargetKindEnumValues;
  static DeletionInfoTargetKindEnum valueOf(String name) =>
      _$deletionInfoTargetKindEnumValueOf(name);
}

/// `live` before a deletion (a preview), `pending` while it can be undone, `final` after `undo_until`.
class DeletionInfoStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'live')
  static const DeletionInfoStateEnum live = _$deletionInfoStateEnum_live;
  @BuiltValueEnumConst(wireName: r'pending')
  static const DeletionInfoStateEnum pending = _$deletionInfoStateEnum_pending;
  @BuiltValueEnumConst(wireName: r'final')
  static const DeletionInfoStateEnum final_ = _$deletionInfoStateEnum_final_;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const DeletionInfoStateEnum unknownDefaultOpenApi =
      _$deletionInfoStateEnum_unknownDefaultOpenApi;

  static Serializer<DeletionInfoStateEnum> get serializer =>
      _$deletionInfoStateEnumSerializer;

  const DeletionInfoStateEnum._(String name) : super(name);

  static BuiltSet<DeletionInfoStateEnum> get values =>
      _$deletionInfoStateEnumValues;
  static DeletionInfoStateEnum valueOf(String name) =>
      _$deletionInfoStateEnumValueOf(name);
}
