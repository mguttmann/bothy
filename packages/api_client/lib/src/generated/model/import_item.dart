//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/import_item_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_item.g.dart';

/// ImportItem
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [originKey]
/// * [itemKind]
/// * [targetKind]
/// * [targetId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [error]
/// * [sourceChanged] - Present and `true` when a repeated key came with a `content_sha256` different from the stored one; nothing was changed (22, 7.2 item 4).
/// * [session] - Present on a `thread` item that carried `earlier_session` and has a thread (22, 9 step 1): `taken_over` when the earlier `agent_sessions` row was written by this batch, `already_present` when a row of the same thread, agent and reference with `end_reason = import` existed, `dropped_late` when the thread has had a current session since the import and nothing was written (22, 9 step 6).
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class ImportItem implements Built<ImportItem, ImportItemBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'origin_key')
  String get originKey;

  @BuiltValueField(wireName: r'item_kind')
  ImportItemItemKindEnum get itemKind;
  // enum itemKindEnum {  space,  channel,  forum_tag,  thread,  message,  file,  user,  reaction,  pin,  };

  @BuiltValueField(wireName: r'target_kind')
  String? get targetKind;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'target_id')
  String? get targetId;

  @BuiltValueField(wireName: r'state')
  ImportItemState get state;
  // enum stateEnum {  pending,  posted,  skipped,  failed,  };

  @BuiltValueField(wireName: r'error')
  String? get error;

  /// Present and `true` when a repeated key came with a `content_sha256` different from the stored one; nothing was changed (22, 7.2 item 4).
  @BuiltValueField(wireName: r'source_changed')
  bool? get sourceChanged;

  /// Present on a `thread` item that carried `earlier_session` and has a thread (22, 9 step 1): `taken_over` when the earlier `agent_sessions` row was written by this batch, `already_present` when a row of the same thread, agent and reference with `end_reason = import` existed, `dropped_late` when the thread has had a current session since the import and nothing was written (22, 9 step 6).
  @BuiltValueField(wireName: r'session')
  ImportItemSessionEnum? get session;
  // enum sessionEnum {  taken_over,  already_present,  dropped_late,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  ImportItem._();

  factory ImportItem([void updates(ImportItemBuilder b)]) = _$ImportItem;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportItemBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportItem> get serializer => _$ImportItemSerializer();
}

class _$ImportItemSerializer implements PrimitiveSerializer<ImportItem> {
  @override
  final Iterable<Type> types = const [ImportItem, _$ImportItem];

  @override
  final String wireName = r'ImportItem';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportItem object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'origin_key';
    yield serializers.serialize(
      object.originKey,
      specifiedType: const FullType(String),
    );
    yield r'item_kind';
    yield serializers.serialize(
      object.itemKind,
      specifiedType: const FullType(ImportItemItemKindEnum),
    );
    if (object.targetKind != null) {
      yield r'target_kind';
      yield serializers.serialize(
        object.targetKind,
        specifiedType: const FullType(String),
      );
    }
    if (object.targetId != null) {
      yield r'target_id';
      yield serializers.serialize(
        object.targetId,
        specifiedType: const FullType(String),
      );
    }
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(ImportItemState),
    );
    if (object.error != null) {
      yield r'error';
      yield serializers.serialize(
        object.error,
        specifiedType: const FullType(String),
      );
    }
    if (object.sourceChanged != null) {
      yield r'source_changed';
      yield serializers.serialize(
        object.sourceChanged,
        specifiedType: const FullType(bool),
      );
    }
    if (object.session != null) {
      yield r'session';
      yield serializers.serialize(
        object.session,
        specifiedType: const FullType(ImportItemSessionEnum),
      );
    }
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportItem object, {
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
    required ImportItemBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.originKey = valueDes;
          break;
        case r'item_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportItemItemKindEnum),
          ) as ImportItemItemKindEnum;
          result.itemKind = valueDes;
          break;
        case r'target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.targetKind = valueDes;
          break;
        case r'target_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.targetId = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportItemState),
          ) as ImportItemState;
          result.state = valueDes;
          break;
        case r'error':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.error = valueDes;
          break;
        case r'source_changed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.sourceChanged = valueDes;
          break;
        case r'session':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ImportItemSessionEnum),
          ) as ImportItemSessionEnum?;
          if (valueDes == null) continue;
          result.session = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportItem deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportItemBuilder();
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

class ImportItemItemKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'space')
  static const ImportItemItemKindEnum space = _$importItemItemKindEnum_space;
  @BuiltValueEnumConst(wireName: r'channel')
  static const ImportItemItemKindEnum channel =
      _$importItemItemKindEnum_channel;
  @BuiltValueEnumConst(wireName: r'forum_tag')
  static const ImportItemItemKindEnum forumTag =
      _$importItemItemKindEnum_forumTag;
  @BuiltValueEnumConst(wireName: r'thread')
  static const ImportItemItemKindEnum thread = _$importItemItemKindEnum_thread;
  @BuiltValueEnumConst(wireName: r'message')
  static const ImportItemItemKindEnum message =
      _$importItemItemKindEnum_message;
  @BuiltValueEnumConst(wireName: r'file')
  static const ImportItemItemKindEnum file = _$importItemItemKindEnum_file;
  @BuiltValueEnumConst(wireName: r'user')
  static const ImportItemItemKindEnum user = _$importItemItemKindEnum_user;
  @BuiltValueEnumConst(wireName: r'reaction')
  static const ImportItemItemKindEnum reaction =
      _$importItemItemKindEnum_reaction;
  @BuiltValueEnumConst(wireName: r'pin')
  static const ImportItemItemKindEnum pin = _$importItemItemKindEnum_pin;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportItemItemKindEnum unknownDefaultOpenApi =
      _$importItemItemKindEnum_unknownDefaultOpenApi;

  static Serializer<ImportItemItemKindEnum> get serializer =>
      _$importItemItemKindEnumSerializer;

  const ImportItemItemKindEnum._(String name) : super(name);

  static BuiltSet<ImportItemItemKindEnum> get values =>
      _$importItemItemKindEnumValues;
  static ImportItemItemKindEnum valueOf(String name) =>
      _$importItemItemKindEnumValueOf(name);
}

/// Present on a `thread` item that carried `earlier_session` and has a thread (22, 9 step 1): `taken_over` when the earlier `agent_sessions` row was written by this batch, `already_present` when a row of the same thread, agent and reference with `end_reason = import` existed, `dropped_late` when the thread has had a current session since the import and nothing was written (22, 9 step 6).
class ImportItemSessionEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'taken_over')
  static const ImportItemSessionEnum takenOver =
      _$importItemSessionEnum_takenOver;
  @BuiltValueEnumConst(wireName: r'already_present')
  static const ImportItemSessionEnum alreadyPresent =
      _$importItemSessionEnum_alreadyPresent;
  @BuiltValueEnumConst(wireName: r'dropped_late')
  static const ImportItemSessionEnum droppedLate =
      _$importItemSessionEnum_droppedLate;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportItemSessionEnum unknownDefaultOpenApi =
      _$importItemSessionEnum_unknownDefaultOpenApi;

  static Serializer<ImportItemSessionEnum> get serializer =>
      _$importItemSessionEnumSerializer;

  const ImportItemSessionEnum._(String name) : super(name);

  static BuiltSet<ImportItemSessionEnum> get values =>
      _$importItemSessionEnumValues;
  static ImportItemSessionEnum valueOf(String name) =>
      _$importItemSessionEnumValueOf(name);
}
