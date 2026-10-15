// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'file.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const FilePurposeEnum _$filePurposeEnum_attachment = const FilePurposeEnum._(
  'attachment',
);
const FilePurposeEnum _$filePurposeEnum_avatar = const FilePurposeEnum._(
  'avatar',
);
const FilePurposeEnum _$filePurposeEnum_branding = const FilePurposeEnum._(
  'branding',
);
const FilePurposeEnum _$filePurposeEnum_gdprExport = const FilePurposeEnum._(
  'gdprExport',
);
const FilePurposeEnum _$filePurposeEnum_unknownDefaultOpenApi =
    const FilePurposeEnum._('unknownDefaultOpenApi');

FilePurposeEnum _$filePurposeEnumValueOf(String name) {
  switch (name) {
    case 'attachment':
      return _$filePurposeEnum_attachment;
    case 'avatar':
      return _$filePurposeEnum_avatar;
    case 'branding':
      return _$filePurposeEnum_branding;
    case 'gdprExport':
      return _$filePurposeEnum_gdprExport;
    case 'unknownDefaultOpenApi':
      return _$filePurposeEnum_unknownDefaultOpenApi;
    default:
      return _$filePurposeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<FilePurposeEnum> _$filePurposeEnumValues =
    BuiltSet<FilePurposeEnum>(const <FilePurposeEnum>[
      _$filePurposeEnum_attachment,
      _$filePurposeEnum_avatar,
      _$filePurposeEnum_branding,
      _$filePurposeEnum_gdprExport,
      _$filePurposeEnum_unknownDefaultOpenApi,
    ]);

const FileUploadStateEnum _$fileUploadStateEnum_pending =
    const FileUploadStateEnum._('pending');
const FileUploadStateEnum _$fileUploadStateEnum_complete =
    const FileUploadStateEnum._('complete');
const FileUploadStateEnum _$fileUploadStateEnum_failed =
    const FileUploadStateEnum._('failed');
const FileUploadStateEnum _$fileUploadStateEnum_unknownDefaultOpenApi =
    const FileUploadStateEnum._('unknownDefaultOpenApi');

FileUploadStateEnum _$fileUploadStateEnumValueOf(String name) {
  switch (name) {
    case 'pending':
      return _$fileUploadStateEnum_pending;
    case 'complete':
      return _$fileUploadStateEnum_complete;
    case 'failed':
      return _$fileUploadStateEnum_failed;
    case 'unknownDefaultOpenApi':
      return _$fileUploadStateEnum_unknownDefaultOpenApi;
    default:
      return _$fileUploadStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<FileUploadStateEnum> _$fileUploadStateEnumValues =
    BuiltSet<FileUploadStateEnum>(const <FileUploadStateEnum>[
      _$fileUploadStateEnum_pending,
      _$fileUploadStateEnum_complete,
      _$fileUploadStateEnum_failed,
      _$fileUploadStateEnum_unknownDefaultOpenApi,
    ]);

const FileScanStatusEnum _$fileScanStatusEnum_pending =
    const FileScanStatusEnum._('pending');
const FileScanStatusEnum _$fileScanStatusEnum_clean =
    const FileScanStatusEnum._('clean');
const FileScanStatusEnum _$fileScanStatusEnum_infected =
    const FileScanStatusEnum._('infected');
const FileScanStatusEnum _$fileScanStatusEnum_error =
    const FileScanStatusEnum._('error');
const FileScanStatusEnum _$fileScanStatusEnum_skipped =
    const FileScanStatusEnum._('skipped');
const FileScanStatusEnum _$fileScanStatusEnum_unknownDefaultOpenApi =
    const FileScanStatusEnum._('unknownDefaultOpenApi');

FileScanStatusEnum _$fileScanStatusEnumValueOf(String name) {
  switch (name) {
    case 'pending':
      return _$fileScanStatusEnum_pending;
    case 'clean':
      return _$fileScanStatusEnum_clean;
    case 'infected':
      return _$fileScanStatusEnum_infected;
    case 'error':
      return _$fileScanStatusEnum_error;
    case 'skipped':
      return _$fileScanStatusEnum_skipped;
    case 'unknownDefaultOpenApi':
      return _$fileScanStatusEnum_unknownDefaultOpenApi;
    default:
      return _$fileScanStatusEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<FileScanStatusEnum> _$fileScanStatusEnumValues =
    BuiltSet<FileScanStatusEnum>(const <FileScanStatusEnum>[
      _$fileScanStatusEnum_pending,
      _$fileScanStatusEnum_clean,
      _$fileScanStatusEnum_infected,
      _$fileScanStatusEnum_error,
      _$fileScanStatusEnum_skipped,
      _$fileScanStatusEnum_unknownDefaultOpenApi,
    ]);

Serializer<FilePurposeEnum> _$filePurposeEnumSerializer =
    _$FilePurposeEnumSerializer();
Serializer<FileUploadStateEnum> _$fileUploadStateEnumSerializer =
    _$FileUploadStateEnumSerializer();
Serializer<FileScanStatusEnum> _$fileScanStatusEnumSerializer =
    _$FileScanStatusEnumSerializer();

class _$FilePurposeEnumSerializer
    implements PrimitiveSerializer<FilePurposeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'attachment': 'attachment',
    'avatar': 'avatar',
    'branding': 'branding',
    'gdprExport': 'gdpr_export',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'attachment': 'attachment',
    'avatar': 'avatar',
    'branding': 'branding',
    'gdpr_export': 'gdprExport',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[FilePurposeEnum];
  @override
  final String wireName = 'FilePurposeEnum';

  @override
  Object serialize(
    Serializers serializers,
    FilePurposeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  FilePurposeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => FilePurposeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$FileUploadStateEnumSerializer
    implements PrimitiveSerializer<FileUploadStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'complete': 'complete',
    'failed': 'failed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'complete': 'complete',
    'failed': 'failed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[FileUploadStateEnum];
  @override
  final String wireName = 'FileUploadStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    FileUploadStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  FileUploadStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => FileUploadStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$FileScanStatusEnumSerializer
    implements PrimitiveSerializer<FileScanStatusEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'clean': 'clean',
    'infected': 'infected',
    'error': 'error',
    'skipped': 'skipped',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'clean': 'clean',
    'infected': 'infected',
    'error': 'error',
    'skipped': 'skipped',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[FileScanStatusEnum];
  @override
  final String wireName = 'FileScanStatusEnum';

  @override
  Object serialize(
    Serializers serializers,
    FileScanStatusEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  FileScanStatusEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => FileScanStatusEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$File extends File {
  @override
  final String id;
  @override
  final String? sha256;
  @override
  final int sizeBytes;
  @override
  final String mimeType;
  @override
  final String originalName;
  @override
  final String uploadedBy;
  @override
  final FilePurposeEnum purpose;
  @override
  final String? scopeChannelId;
  @override
  final FileUploadStateEnum uploadState;
  @override
  final FileScanStatusEnum scanStatus;
  @override
  final int? width;
  @override
  final int? height;
  @override
  final int? durationMs;
  @override
  final TranscriptStatus transcriptStatus;
  @override
  final String? transcriptLanguage;
  @override
  final TranscriptSource? transcriptSource;
  @override
  final BuiltList<Thumbnail> thumbnails;
  @override
  final DateTime createdAt;

  factory _$File([void Function(FileBuilder)? updates]) =>
      (FileBuilder()..update(updates))._build();

  _$File._({
    required this.id,
    this.sha256,
    required this.sizeBytes,
    required this.mimeType,
    required this.originalName,
    required this.uploadedBy,
    required this.purpose,
    this.scopeChannelId,
    required this.uploadState,
    required this.scanStatus,
    this.width,
    this.height,
    this.durationMs,
    required this.transcriptStatus,
    this.transcriptLanguage,
    this.transcriptSource,
    required this.thumbnails,
    required this.createdAt,
  }) : super._();
  @override
  File rebuild(void Function(FileBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FileBuilder toBuilder() => FileBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is File &&
        id == other.id &&
        sha256 == other.sha256 &&
        sizeBytes == other.sizeBytes &&
        mimeType == other.mimeType &&
        originalName == other.originalName &&
        uploadedBy == other.uploadedBy &&
        purpose == other.purpose &&
        scopeChannelId == other.scopeChannelId &&
        uploadState == other.uploadState &&
        scanStatus == other.scanStatus &&
        width == other.width &&
        height == other.height &&
        durationMs == other.durationMs &&
        transcriptStatus == other.transcriptStatus &&
        transcriptLanguage == other.transcriptLanguage &&
        transcriptSource == other.transcriptSource &&
        thumbnails == other.thumbnails &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, sha256.hashCode);
    _$hash = $jc(_$hash, sizeBytes.hashCode);
    _$hash = $jc(_$hash, mimeType.hashCode);
    _$hash = $jc(_$hash, originalName.hashCode);
    _$hash = $jc(_$hash, uploadedBy.hashCode);
    _$hash = $jc(_$hash, purpose.hashCode);
    _$hash = $jc(_$hash, scopeChannelId.hashCode);
    _$hash = $jc(_$hash, uploadState.hashCode);
    _$hash = $jc(_$hash, scanStatus.hashCode);
    _$hash = $jc(_$hash, width.hashCode);
    _$hash = $jc(_$hash, height.hashCode);
    _$hash = $jc(_$hash, durationMs.hashCode);
    _$hash = $jc(_$hash, transcriptStatus.hashCode);
    _$hash = $jc(_$hash, transcriptLanguage.hashCode);
    _$hash = $jc(_$hash, transcriptSource.hashCode);
    _$hash = $jc(_$hash, thumbnails.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'File')
          ..add('id', id)
          ..add('sha256', sha256)
          ..add('sizeBytes', sizeBytes)
          ..add('mimeType', mimeType)
          ..add('originalName', originalName)
          ..add('uploadedBy', uploadedBy)
          ..add('purpose', purpose)
          ..add('scopeChannelId', scopeChannelId)
          ..add('uploadState', uploadState)
          ..add('scanStatus', scanStatus)
          ..add('width', width)
          ..add('height', height)
          ..add('durationMs', durationMs)
          ..add('transcriptStatus', transcriptStatus)
          ..add('transcriptLanguage', transcriptLanguage)
          ..add('transcriptSource', transcriptSource)
          ..add('thumbnails', thumbnails)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class FileBuilder implements Builder<File, FileBuilder> {
  _$File? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _sha256;
  String? get sha256 => _$this._sha256;
  set sha256(String? sha256) => _$this._sha256 = sha256;

  int? _sizeBytes;
  int? get sizeBytes => _$this._sizeBytes;
  set sizeBytes(int? sizeBytes) => _$this._sizeBytes = sizeBytes;

  String? _mimeType;
  String? get mimeType => _$this._mimeType;
  set mimeType(String? mimeType) => _$this._mimeType = mimeType;

  String? _originalName;
  String? get originalName => _$this._originalName;
  set originalName(String? originalName) => _$this._originalName = originalName;

  String? _uploadedBy;
  String? get uploadedBy => _$this._uploadedBy;
  set uploadedBy(String? uploadedBy) => _$this._uploadedBy = uploadedBy;

  FilePurposeEnum? _purpose;
  FilePurposeEnum? get purpose => _$this._purpose;
  set purpose(FilePurposeEnum? purpose) => _$this._purpose = purpose;

  String? _scopeChannelId;
  String? get scopeChannelId => _$this._scopeChannelId;
  set scopeChannelId(String? scopeChannelId) =>
      _$this._scopeChannelId = scopeChannelId;

  FileUploadStateEnum? _uploadState;
  FileUploadStateEnum? get uploadState => _$this._uploadState;
  set uploadState(FileUploadStateEnum? uploadState) =>
      _$this._uploadState = uploadState;

  FileScanStatusEnum? _scanStatus;
  FileScanStatusEnum? get scanStatus => _$this._scanStatus;
  set scanStatus(FileScanStatusEnum? scanStatus) =>
      _$this._scanStatus = scanStatus;

  int? _width;
  int? get width => _$this._width;
  set width(int? width) => _$this._width = width;

  int? _height;
  int? get height => _$this._height;
  set height(int? height) => _$this._height = height;

  int? _durationMs;
  int? get durationMs => _$this._durationMs;
  set durationMs(int? durationMs) => _$this._durationMs = durationMs;

  TranscriptStatus? _transcriptStatus;
  TranscriptStatus? get transcriptStatus => _$this._transcriptStatus;
  set transcriptStatus(TranscriptStatus? transcriptStatus) =>
      _$this._transcriptStatus = transcriptStatus;

  String? _transcriptLanguage;
  String? get transcriptLanguage => _$this._transcriptLanguage;
  set transcriptLanguage(String? transcriptLanguage) =>
      _$this._transcriptLanguage = transcriptLanguage;

  TranscriptSource? _transcriptSource;
  TranscriptSource? get transcriptSource => _$this._transcriptSource;
  set transcriptSource(TranscriptSource? transcriptSource) =>
      _$this._transcriptSource = transcriptSource;

  ListBuilder<Thumbnail>? _thumbnails;
  ListBuilder<Thumbnail> get thumbnails =>
      _$this._thumbnails ??= ListBuilder<Thumbnail>();
  set thumbnails(ListBuilder<Thumbnail>? thumbnails) =>
      _$this._thumbnails = thumbnails;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  FileBuilder() {
    File._defaults(this);
  }

  FileBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _sha256 = $v.sha256;
      _sizeBytes = $v.sizeBytes;
      _mimeType = $v.mimeType;
      _originalName = $v.originalName;
      _uploadedBy = $v.uploadedBy;
      _purpose = $v.purpose;
      _scopeChannelId = $v.scopeChannelId;
      _uploadState = $v.uploadState;
      _scanStatus = $v.scanStatus;
      _width = $v.width;
      _height = $v.height;
      _durationMs = $v.durationMs;
      _transcriptStatus = $v.transcriptStatus;
      _transcriptLanguage = $v.transcriptLanguage;
      _transcriptSource = $v.transcriptSource;
      _thumbnails = $v.thumbnails.toBuilder();
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(File other) {
    _$v = other as _$File;
  }

  @override
  void update(void Function(FileBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  File build() => _build();

  _$File _build() {
    _$File _$result;
    try {
      _$result =
          _$v ??
          _$File._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'File', 'id'),
            sha256: sha256,
            sizeBytes: BuiltValueNullFieldError.checkNotNull(
              sizeBytes,
              r'File',
              'sizeBytes',
            ),
            mimeType: BuiltValueNullFieldError.checkNotNull(
              mimeType,
              r'File',
              'mimeType',
            ),
            originalName: BuiltValueNullFieldError.checkNotNull(
              originalName,
              r'File',
              'originalName',
            ),
            uploadedBy: BuiltValueNullFieldError.checkNotNull(
              uploadedBy,
              r'File',
              'uploadedBy',
            ),
            purpose: BuiltValueNullFieldError.checkNotNull(
              purpose,
              r'File',
              'purpose',
            ),
            scopeChannelId: scopeChannelId,
            uploadState: BuiltValueNullFieldError.checkNotNull(
              uploadState,
              r'File',
              'uploadState',
            ),
            scanStatus: BuiltValueNullFieldError.checkNotNull(
              scanStatus,
              r'File',
              'scanStatus',
            ),
            width: width,
            height: height,
            durationMs: durationMs,
            transcriptStatus: BuiltValueNullFieldError.checkNotNull(
              transcriptStatus,
              r'File',
              'transcriptStatus',
            ),
            transcriptLanguage: transcriptLanguage,
            transcriptSource: transcriptSource,
            thumbnails: thumbnails.build(),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'File',
              'createdAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'thumbnails';
        thumbnails.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'File', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
