// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'upload_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const UploadStateUploadStateEnum _$uploadStateUploadStateEnum_pending =
    const UploadStateUploadStateEnum._('pending');
const UploadStateUploadStateEnum _$uploadStateUploadStateEnum_uploading =
    const UploadStateUploadStateEnum._('uploading');
const UploadStateUploadStateEnum _$uploadStateUploadStateEnum_complete =
    const UploadStateUploadStateEnum._('complete');
const UploadStateUploadStateEnum
_$uploadStateUploadStateEnum_unknownDefaultOpenApi =
    const UploadStateUploadStateEnum._('unknownDefaultOpenApi');

UploadStateUploadStateEnum _$uploadStateUploadStateEnumValueOf(String name) {
  switch (name) {
    case 'pending':
      return _$uploadStateUploadStateEnum_pending;
    case 'uploading':
      return _$uploadStateUploadStateEnum_uploading;
    case 'complete':
      return _$uploadStateUploadStateEnum_complete;
    case 'unknownDefaultOpenApi':
      return _$uploadStateUploadStateEnum_unknownDefaultOpenApi;
    default:
      return _$uploadStateUploadStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<UploadStateUploadStateEnum> _$uploadStateUploadStateEnumValues =
    BuiltSet<UploadStateUploadStateEnum>(const <UploadStateUploadStateEnum>[
      _$uploadStateUploadStateEnum_pending,
      _$uploadStateUploadStateEnum_uploading,
      _$uploadStateUploadStateEnum_complete,
      _$uploadStateUploadStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<UploadStateUploadStateEnum> _$uploadStateUploadStateEnumSerializer =
    _$UploadStateUploadStateEnumSerializer();

class _$UploadStateUploadStateEnumSerializer
    implements PrimitiveSerializer<UploadStateUploadStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'uploading': 'uploading',
    'complete': 'complete',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'uploading': 'uploading',
    'complete': 'complete',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[UploadStateUploadStateEnum];
  @override
  final String wireName = 'UploadStateUploadStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    UploadStateUploadStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  UploadStateUploadStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => UploadStateUploadStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$UploadState extends UploadState {
  @override
  final String fileId;
  @override
  final UploadStateUploadStateEnum uploadState;
  @override
  final int partSizeBytes;
  @override
  final BuiltList<UploadStateCompletedPartsInner> completedParts;
  @override
  final DateTime expiresAt;

  factory _$UploadState([void Function(UploadStateBuilder)? updates]) =>
      (UploadStateBuilder()..update(updates))._build();

  _$UploadState._({
    required this.fileId,
    required this.uploadState,
    required this.partSizeBytes,
    required this.completedParts,
    required this.expiresAt,
  }) : super._();
  @override
  UploadState rebuild(void Function(UploadStateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UploadStateBuilder toBuilder() => UploadStateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UploadState &&
        fileId == other.fileId &&
        uploadState == other.uploadState &&
        partSizeBytes == other.partSizeBytes &&
        completedParts == other.completedParts &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, fileId.hashCode);
    _$hash = $jc(_$hash, uploadState.hashCode);
    _$hash = $jc(_$hash, partSizeBytes.hashCode);
    _$hash = $jc(_$hash, completedParts.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UploadState')
          ..add('fileId', fileId)
          ..add('uploadState', uploadState)
          ..add('partSizeBytes', partSizeBytes)
          ..add('completedParts', completedParts)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class UploadStateBuilder implements Builder<UploadState, UploadStateBuilder> {
  _$UploadState? _$v;

  String? _fileId;
  String? get fileId => _$this._fileId;
  set fileId(String? fileId) => _$this._fileId = fileId;

  UploadStateUploadStateEnum? _uploadState;
  UploadStateUploadStateEnum? get uploadState => _$this._uploadState;
  set uploadState(UploadStateUploadStateEnum? uploadState) =>
      _$this._uploadState = uploadState;

  int? _partSizeBytes;
  int? get partSizeBytes => _$this._partSizeBytes;
  set partSizeBytes(int? partSizeBytes) =>
      _$this._partSizeBytes = partSizeBytes;

  ListBuilder<UploadStateCompletedPartsInner>? _completedParts;
  ListBuilder<UploadStateCompletedPartsInner> get completedParts =>
      _$this._completedParts ??= ListBuilder<UploadStateCompletedPartsInner>();
  set completedParts(
    ListBuilder<UploadStateCompletedPartsInner>? completedParts,
  ) => _$this._completedParts = completedParts;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  UploadStateBuilder() {
    UploadState._defaults(this);
  }

  UploadStateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _fileId = $v.fileId;
      _uploadState = $v.uploadState;
      _partSizeBytes = $v.partSizeBytes;
      _completedParts = $v.completedParts.toBuilder();
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UploadState other) {
    _$v = other as _$UploadState;
  }

  @override
  void update(void Function(UploadStateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UploadState build() => _build();

  _$UploadState _build() {
    _$UploadState _$result;
    try {
      _$result =
          _$v ??
          _$UploadState._(
            fileId: BuiltValueNullFieldError.checkNotNull(
              fileId,
              r'UploadState',
              'fileId',
            ),
            uploadState: BuiltValueNullFieldError.checkNotNull(
              uploadState,
              r'UploadState',
              'uploadState',
            ),
            partSizeBytes: BuiltValueNullFieldError.checkNotNull(
              partSizeBytes,
              r'UploadState',
              'partSizeBytes',
            ),
            completedParts: completedParts.build(),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'UploadState',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'completedParts';
        completedParts.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'UploadState',
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
