//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/transcript_status.dart';
import 'package:api_client/src/generated/model/transcript_source.dart';
import 'package:api_client/src/generated/model/thumbnail.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'file.g.dart';

/// File
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [sha256] - Hex SHA-256 of the stored bytes.
/// * [sizeBytes]
/// * [mimeType] - Detected by the server.
/// * [originalName]
/// * [uploadedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [purpose]
/// * [scopeChannelId] - Channel the file was uploaded for; present only before the file is attached to a message. An attached file is read through the channel of its message.
/// * [uploadState]
/// * [scanStatus]
/// * [width]
/// * [height]
/// * [durationMs]
/// * [transcriptStatus]
/// * [transcriptLanguage]
/// * [transcriptSource]
/// * [thumbnails]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class File implements Built<File, FileBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// Hex SHA-256 of the stored bytes.
  @BuiltValueField(wireName: r'sha256')
  String? get sha256;

  @BuiltValueField(wireName: r'size_bytes')
  int get sizeBytes;

  /// Detected by the server.
  @BuiltValueField(wireName: r'mime_type')
  String get mimeType;

  @BuiltValueField(wireName: r'original_name')
  String get originalName;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'uploaded_by')
  String get uploadedBy;

  @BuiltValueField(wireName: r'purpose')
  FilePurposeEnum get purpose;
  // enum purposeEnum {  attachment,  avatar,  branding,  gdpr_export,  };

  /// Channel the file was uploaded for; present only before the file is attached to a message. An attached file is read through the channel of its message.
  @BuiltValueField(wireName: r'scope_channel_id')
  String? get scopeChannelId;

  @BuiltValueField(wireName: r'upload_state')
  FileUploadStateEnum get uploadState;
  // enum uploadStateEnum {  pending,  complete,  failed,  };

  @BuiltValueField(wireName: r'scan_status')
  FileScanStatusEnum get scanStatus;
  // enum scanStatusEnum {  pending,  clean,  infected,  error,  skipped,  };

  @BuiltValueField(wireName: r'width')
  int? get width;

  @BuiltValueField(wireName: r'height')
  int? get height;

  @BuiltValueField(wireName: r'duration_ms')
  int? get durationMs;

  @BuiltValueField(wireName: r'transcript_status')
  TranscriptStatus get transcriptStatus;
  // enum transcriptStatusEnum {  none,  pending,  done,  failed,  };

  @BuiltValueField(wireName: r'transcript_language')
  String? get transcriptLanguage;

  @BuiltValueField(wireName: r'transcript_source')
  TranscriptSource? get transcriptSource;
  // enum transcriptSourceEnum {  agent,  service,  };

  @BuiltValueField(wireName: r'thumbnails')
  BuiltList<Thumbnail> get thumbnails;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  File._();

  factory File([void updates(FileBuilder b)]) = _$File;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(FileBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<File> get serializer => _$FileSerializer();
}

class _$FileSerializer implements PrimitiveSerializer<File> {
  @override
  final Iterable<Type> types = const [File, _$File];

  @override
  final String wireName = r'File';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    File object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    if (object.sha256 != null) {
      yield r'sha256';
      yield serializers.serialize(
        object.sha256,
        specifiedType: const FullType(String),
      );
    }
    yield r'size_bytes';
    yield serializers.serialize(
      object.sizeBytes,
      specifiedType: const FullType(int),
    );
    yield r'mime_type';
    yield serializers.serialize(
      object.mimeType,
      specifiedType: const FullType(String),
    );
    yield r'original_name';
    yield serializers.serialize(
      object.originalName,
      specifiedType: const FullType(String),
    );
    yield r'uploaded_by';
    yield serializers.serialize(
      object.uploadedBy,
      specifiedType: const FullType(String),
    );
    yield r'purpose';
    yield serializers.serialize(
      object.purpose,
      specifiedType: const FullType(FilePurposeEnum),
    );
    if (object.scopeChannelId != null) {
      yield r'scope_channel_id';
      yield serializers.serialize(
        object.scopeChannelId,
        specifiedType: const FullType(String),
      );
    }
    yield r'upload_state';
    yield serializers.serialize(
      object.uploadState,
      specifiedType: const FullType(FileUploadStateEnum),
    );
    yield r'scan_status';
    yield serializers.serialize(
      object.scanStatus,
      specifiedType: const FullType(FileScanStatusEnum),
    );
    if (object.width != null) {
      yield r'width';
      yield serializers.serialize(
        object.width,
        specifiedType: const FullType(int),
      );
    }
    if (object.height != null) {
      yield r'height';
      yield serializers.serialize(
        object.height,
        specifiedType: const FullType(int),
      );
    }
    if (object.durationMs != null) {
      yield r'duration_ms';
      yield serializers.serialize(
        object.durationMs,
        specifiedType: const FullType(int),
      );
    }
    yield r'transcript_status';
    yield serializers.serialize(
      object.transcriptStatus,
      specifiedType: const FullType(TranscriptStatus),
    );
    if (object.transcriptLanguage != null) {
      yield r'transcript_language';
      yield serializers.serialize(
        object.transcriptLanguage,
        specifiedType: const FullType(String),
      );
    }
    if (object.transcriptSource != null) {
      yield r'transcript_source';
      yield serializers.serialize(
        object.transcriptSource,
        specifiedType: const FullType(TranscriptSource),
      );
    }
    yield r'thumbnails';
    yield serializers.serialize(
      object.thumbnails,
      specifiedType: const FullType(BuiltList, [FullType(Thumbnail)]),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    File object, {
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
    required FileBuilder result,
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
        case r'sha256':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.sha256 = valueDes;
          break;
        case r'size_bytes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.sizeBytes = valueDes;
          break;
        case r'mime_type':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.mimeType = valueDes;
          break;
        case r'original_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.originalName = valueDes;
          break;
        case r'uploaded_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.uploadedBy = valueDes;
          break;
        case r'purpose':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(FilePurposeEnum),
          ) as FilePurposeEnum;
          result.purpose = valueDes;
          break;
        case r'scope_channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.scopeChannelId = valueDes;
          break;
        case r'upload_state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(FileUploadStateEnum),
          ) as FileUploadStateEnum;
          result.uploadState = valueDes;
          break;
        case r'scan_status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(FileScanStatusEnum),
          ) as FileScanStatusEnum;
          result.scanStatus = valueDes;
          break;
        case r'width':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.width = valueDes;
          break;
        case r'height':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.height = valueDes;
          break;
        case r'duration_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.durationMs = valueDes;
          break;
        case r'transcript_status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TranscriptStatus),
          ) as TranscriptStatus;
          result.transcriptStatus = valueDes;
          break;
        case r'transcript_language':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.transcriptLanguage = valueDes;
          break;
        case r'transcript_source':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TranscriptSource),
          ) as TranscriptSource?;
          if (valueDes == null) continue;
          result.transcriptSource = valueDes;
          break;
        case r'thumbnails':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Thumbnail)]),
          ) as BuiltList<Thumbnail>;
          result.thumbnails.replace(valueDes);
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
  File deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = FileBuilder();
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

class FilePurposeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'attachment')
  static const FilePurposeEnum attachment = _$filePurposeEnum_attachment;
  @BuiltValueEnumConst(wireName: r'avatar')
  static const FilePurposeEnum avatar = _$filePurposeEnum_avatar;
  @BuiltValueEnumConst(wireName: r'branding')
  static const FilePurposeEnum branding = _$filePurposeEnum_branding;
  @BuiltValueEnumConst(wireName: r'gdpr_export')
  static const FilePurposeEnum gdprExport = _$filePurposeEnum_gdprExport;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const FilePurposeEnum unknownDefaultOpenApi =
      _$filePurposeEnum_unknownDefaultOpenApi;

  static Serializer<FilePurposeEnum> get serializer =>
      _$filePurposeEnumSerializer;

  const FilePurposeEnum._(String name) : super(name);

  static BuiltSet<FilePurposeEnum> get values => _$filePurposeEnumValues;
  static FilePurposeEnum valueOf(String name) => _$filePurposeEnumValueOf(name);
}

class FileUploadStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const FileUploadStateEnum pending = _$fileUploadStateEnum_pending;
  @BuiltValueEnumConst(wireName: r'complete')
  static const FileUploadStateEnum complete = _$fileUploadStateEnum_complete;
  @BuiltValueEnumConst(wireName: r'failed')
  static const FileUploadStateEnum failed = _$fileUploadStateEnum_failed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const FileUploadStateEnum unknownDefaultOpenApi =
      _$fileUploadStateEnum_unknownDefaultOpenApi;

  static Serializer<FileUploadStateEnum> get serializer =>
      _$fileUploadStateEnumSerializer;

  const FileUploadStateEnum._(String name) : super(name);

  static BuiltSet<FileUploadStateEnum> get values =>
      _$fileUploadStateEnumValues;
  static FileUploadStateEnum valueOf(String name) =>
      _$fileUploadStateEnumValueOf(name);
}

class FileScanStatusEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const FileScanStatusEnum pending = _$fileScanStatusEnum_pending;
  @BuiltValueEnumConst(wireName: r'clean')
  static const FileScanStatusEnum clean = _$fileScanStatusEnum_clean;
  @BuiltValueEnumConst(wireName: r'infected')
  static const FileScanStatusEnum infected = _$fileScanStatusEnum_infected;
  @BuiltValueEnumConst(wireName: r'error')
  static const FileScanStatusEnum error = _$fileScanStatusEnum_error;
  @BuiltValueEnumConst(wireName: r'skipped')
  static const FileScanStatusEnum skipped = _$fileScanStatusEnum_skipped;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const FileScanStatusEnum unknownDefaultOpenApi =
      _$fileScanStatusEnum_unknownDefaultOpenApi;

  static Serializer<FileScanStatusEnum> get serializer =>
      _$fileScanStatusEnumSerializer;

  const FileScanStatusEnum._(String name) : super(name);

  static BuiltSet<FileScanStatusEnum> get values => _$fileScanStatusEnumValues;
  static FileScanStatusEnum valueOf(String name) =>
      _$fileScanStatusEnumValueOf(name);
}
