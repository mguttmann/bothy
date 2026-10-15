// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'upload_session.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const UploadSessionModeEnum _$uploadSessionModeEnum_direct =
    const UploadSessionModeEnum._('direct');
const UploadSessionModeEnum _$uploadSessionModeEnum_presigned =
    const UploadSessionModeEnum._('presigned');
const UploadSessionModeEnum _$uploadSessionModeEnum_unknownDefaultOpenApi =
    const UploadSessionModeEnum._('unknownDefaultOpenApi');

UploadSessionModeEnum _$uploadSessionModeEnumValueOf(String name) {
  switch (name) {
    case 'direct':
      return _$uploadSessionModeEnum_direct;
    case 'presigned':
      return _$uploadSessionModeEnum_presigned;
    case 'unknownDefaultOpenApi':
      return _$uploadSessionModeEnum_unknownDefaultOpenApi;
    default:
      return _$uploadSessionModeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<UploadSessionModeEnum> _$uploadSessionModeEnumValues =
    BuiltSet<UploadSessionModeEnum>(const <UploadSessionModeEnum>[
      _$uploadSessionModeEnum_direct,
      _$uploadSessionModeEnum_presigned,
      _$uploadSessionModeEnum_unknownDefaultOpenApi,
    ]);

Serializer<UploadSessionModeEnum> _$uploadSessionModeEnumSerializer =
    _$UploadSessionModeEnumSerializer();

class _$UploadSessionModeEnumSerializer
    implements PrimitiveSerializer<UploadSessionModeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'direct': 'direct',
    'presigned': 'presigned',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'direct': 'direct',
    'presigned': 'presigned',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[UploadSessionModeEnum];
  @override
  final String wireName = 'UploadSessionModeEnum';

  @override
  Object serialize(
    Serializers serializers,
    UploadSessionModeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  UploadSessionModeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => UploadSessionModeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$UploadSession extends UploadSession {
  @override
  final String fileId;
  @override
  final UploadSessionModeEnum mode;
  @override
  final int partSizeBytes;
  @override
  final BuiltList<UploadPart> parts;
  @override
  final DateTime expiresAt;

  factory _$UploadSession([void Function(UploadSessionBuilder)? updates]) =>
      (UploadSessionBuilder()..update(updates))._build();

  _$UploadSession._({
    required this.fileId,
    required this.mode,
    required this.partSizeBytes,
    required this.parts,
    required this.expiresAt,
  }) : super._();
  @override
  UploadSession rebuild(void Function(UploadSessionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UploadSessionBuilder toBuilder() => UploadSessionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UploadSession &&
        fileId == other.fileId &&
        mode == other.mode &&
        partSizeBytes == other.partSizeBytes &&
        parts == other.parts &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, fileId.hashCode);
    _$hash = $jc(_$hash, mode.hashCode);
    _$hash = $jc(_$hash, partSizeBytes.hashCode);
    _$hash = $jc(_$hash, parts.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UploadSession')
          ..add('fileId', fileId)
          ..add('mode', mode)
          ..add('partSizeBytes', partSizeBytes)
          ..add('parts', parts)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class UploadSessionBuilder
    implements Builder<UploadSession, UploadSessionBuilder> {
  _$UploadSession? _$v;

  String? _fileId;
  String? get fileId => _$this._fileId;
  set fileId(String? fileId) => _$this._fileId = fileId;

  UploadSessionModeEnum? _mode;
  UploadSessionModeEnum? get mode => _$this._mode;
  set mode(UploadSessionModeEnum? mode) => _$this._mode = mode;

  int? _partSizeBytes;
  int? get partSizeBytes => _$this._partSizeBytes;
  set partSizeBytes(int? partSizeBytes) =>
      _$this._partSizeBytes = partSizeBytes;

  ListBuilder<UploadPart>? _parts;
  ListBuilder<UploadPart> get parts =>
      _$this._parts ??= ListBuilder<UploadPart>();
  set parts(ListBuilder<UploadPart>? parts) => _$this._parts = parts;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  UploadSessionBuilder() {
    UploadSession._defaults(this);
  }

  UploadSessionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _fileId = $v.fileId;
      _mode = $v.mode;
      _partSizeBytes = $v.partSizeBytes;
      _parts = $v.parts.toBuilder();
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UploadSession other) {
    _$v = other as _$UploadSession;
  }

  @override
  void update(void Function(UploadSessionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UploadSession build() => _build();

  _$UploadSession _build() {
    _$UploadSession _$result;
    try {
      _$result =
          _$v ??
          _$UploadSession._(
            fileId: BuiltValueNullFieldError.checkNotNull(
              fileId,
              r'UploadSession',
              'fileId',
            ),
            mode: BuiltValueNullFieldError.checkNotNull(
              mode,
              r'UploadSession',
              'mode',
            ),
            partSizeBytes: BuiltValueNullFieldError.checkNotNull(
              partSizeBytes,
              r'UploadSession',
              'partSizeBytes',
            ),
            parts: parts.build(),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'UploadSession',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'parts';
        parts.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'UploadSession',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
