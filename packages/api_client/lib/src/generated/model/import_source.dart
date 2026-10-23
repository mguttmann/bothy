//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/import_kind.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_source.g.dart';

/// ImportSource
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [kind]
/// * [name]
/// * [config] - Schema `import_source.<kind>.v1`; credentials only as `secret_ref` names.
/// * [state] - A dry run writes nothing and keeps the state.
/// * [stats] - Counters without content. The server's counters per item kind and state are under `server` (`server.<item_kind>.<state>`, `server.source_changed`, `server.cancelled`); the counters of the import tool from `setImportCursor` are under `tool`.
/// * [cursor] - The object last stored by `setImportCursor` (progress and the run lease of the import tool, 22 7.3 and 7.4); `{}` before the first one. The server never interprets it.
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [startedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [finishedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class ImportSource
    implements Built<ImportSource, ImportSourceBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'kind')
  ImportKind get kind;
  // enum kindEnum {  discord,  archive,  };

  @BuiltValueField(wireName: r'name')
  String get name;

  /// Schema `import_source.<kind>.v1`; credentials only as `secret_ref` names.
  @BuiltValueField(wireName: r'config')
  BuiltMap<String, JsonObject?> get config;

  /// A dry run writes nothing and keeps the state.
  @BuiltValueField(wireName: r'state')
  ImportSourceStateEnum get state;
  // enum stateEnum {  draft,  running,  paused,  completed,  failed,  };

  /// Counters without content. The server's counters per item kind and state are under `server` (`server.<item_kind>.<state>`, `server.source_changed`, `server.cancelled`); the counters of the import tool from `setImportCursor` are under `tool`.
  @BuiltValueField(wireName: r'stats')
  BuiltMap<String, JsonObject?> get stats;

  /// The object last stored by `setImportCursor` (progress and the run lease of the import tool, 22 7.3 and 7.4); `{}` before the first one. The server never interprets it.
  @BuiltValueField(wireName: r'cursor')
  BuiltMap<String, JsonObject?>? get cursor;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'started_at')
  DateTime? get startedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'finished_at')
  DateTime? get finishedAt;

  ImportSource._();

  factory ImportSource([void updates(ImportSourceBuilder b)]) = _$ImportSource;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportSourceBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportSource> get serializer => _$ImportSourceSerializer();
}

class _$ImportSourceSerializer implements PrimitiveSerializer<ImportSource> {
  @override
  final Iterable<Type> types = const [ImportSource, _$ImportSource];

  @override
  final String wireName = r'ImportSource';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportSource object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(ImportKind),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'config';
    yield serializers.serialize(
      object.config,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(ImportSourceStateEnum),
    );
    yield r'stats';
    yield serializers.serialize(
      object.stats,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    if (object.cursor != null) {
      yield r'cursor';
      yield serializers.serialize(
        object.cursor,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.startedAt != null) {
      yield r'started_at';
      yield serializers.serialize(
        object.startedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.finishedAt != null) {
      yield r'finished_at';
      yield serializers.serialize(
        object.finishedAt,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportSource object, {
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
    required ImportSourceBuilder result,
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
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportKind),
          ) as ImportKind;
          result.kind = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'config':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.config.replace(valueDes);
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ImportSourceStateEnum),
          ) as ImportSourceStateEnum;
          result.state = valueDes;
          break;
        case r'stats':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.stats.replace(valueDes);
          break;
        case r'cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.cursor.replace(valueDes);
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'started_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.startedAt = valueDes;
          break;
        case r'finished_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.finishedAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportSource deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportSourceBuilder();
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

/// A dry run writes nothing and keeps the state.
class ImportSourceStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'draft')
  static const ImportSourceStateEnum draft = _$importSourceStateEnum_draft;
  @BuiltValueEnumConst(wireName: r'running')
  static const ImportSourceStateEnum running = _$importSourceStateEnum_running;
  @BuiltValueEnumConst(wireName: r'paused')
  static const ImportSourceStateEnum paused = _$importSourceStateEnum_paused;
  @BuiltValueEnumConst(wireName: r'completed')
  static const ImportSourceStateEnum completed =
      _$importSourceStateEnum_completed;
  @BuiltValueEnumConst(wireName: r'failed')
  static const ImportSourceStateEnum failed = _$importSourceStateEnum_failed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ImportSourceStateEnum unknownDefaultOpenApi =
      _$importSourceStateEnum_unknownDefaultOpenApi;

  static Serializer<ImportSourceStateEnum> get serializer =>
      _$importSourceStateEnumSerializer;

  const ImportSourceStateEnum._(String name) : super(name);

  static BuiltSet<ImportSourceStateEnum> get values =>
      _$importSourceStateEnumValues;
  static ImportSourceStateEnum valueOf(String name) =>
      _$importSourceStateEnumValueOf(name);
}
