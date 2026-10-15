// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_context_attachment.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunContextAttachment extends RunContextAttachment {
  @override
  final String fileId;
  @override
  final String originalName;
  @override
  final String mimeType;
  @override
  final int sizeBytes;
  @override
  final String scanStatus;
  @override
  final String? transcript;

  factory _$RunContextAttachment([
    void Function(RunContextAttachmentBuilder)? updates,
  ]) => (RunContextAttachmentBuilder()..update(updates))._build();

  _$RunContextAttachment._({
    required this.fileId,
    required this.originalName,
    required this.mimeType,
    required this.sizeBytes,
    required this.scanStatus,
    this.transcript,
  }) : super._();
  @override
  RunContextAttachment rebuild(
    void Function(RunContextAttachmentBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  RunContextAttachmentBuilder toBuilder() =>
      RunContextAttachmentBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunContextAttachment &&
        fileId == other.fileId &&
        originalName == other.originalName &&
        mimeType == other.mimeType &&
        sizeBytes == other.sizeBytes &&
        scanStatus == other.scanStatus &&
        transcript == other.transcript;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, fileId.hashCode);
    _$hash = $jc(_$hash, originalName.hashCode);
    _$hash = $jc(_$hash, mimeType.hashCode);
    _$hash = $jc(_$hash, sizeBytes.hashCode);
    _$hash = $jc(_$hash, scanStatus.hashCode);
    _$hash = $jc(_$hash, transcript.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunContextAttachment')
          ..add('fileId', fileId)
          ..add('originalName', originalName)
          ..add('mimeType', mimeType)
          ..add('sizeBytes', sizeBytes)
          ..add('scanStatus', scanStatus)
          ..add('transcript', transcript))
        .toString();
  }
}

class RunContextAttachmentBuilder
    implements Builder<RunContextAttachment, RunContextAttachmentBuilder> {
  _$RunContextAttachment? _$v;

  String? _fileId;
  String? get fileId => _$this._fileId;
  set fileId(String? fileId) => _$this._fileId = fileId;

  String? _originalName;
  String? get originalName => _$this._originalName;
  set originalName(String? originalName) => _$this._originalName = originalName;

  String? _mimeType;
  String? get mimeType => _$this._mimeType;
  set mimeType(String? mimeType) => _$this._mimeType = mimeType;

  int? _sizeBytes;
  int? get sizeBytes => _$this._sizeBytes;
  set sizeBytes(int? sizeBytes) => _$this._sizeBytes = sizeBytes;

  String? _scanStatus;
  String? get scanStatus => _$this._scanStatus;
  set scanStatus(String? scanStatus) => _$this._scanStatus = scanStatus;

  String? _transcript;
  String? get transcript => _$this._transcript;
  set transcript(String? transcript) => _$this._transcript = transcript;

  RunContextAttachmentBuilder() {
    RunContextAttachment._defaults(this);
  }

  RunContextAttachmentBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _fileId = $v.fileId;
      _originalName = $v.originalName;
      _mimeType = $v.mimeType;
      _sizeBytes = $v.sizeBytes;
      _scanStatus = $v.scanStatus;
      _transcript = $v.transcript;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunContextAttachment other) {
    _$v = other as _$RunContextAttachment;
  }

  @override
  void update(void Function(RunContextAttachmentBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunContextAttachment build() => _build();

  _$RunContextAttachment _build() {
    final _$result =
        _$v ??
        _$RunContextAttachment._(
          fileId: BuiltValueNullFieldError.checkNotNull(
            fileId,
            r'RunContextAttachment',
            'fileId',
          ),
          originalName: BuiltValueNullFieldError.checkNotNull(
            originalName,
            r'RunContextAttachment',
            'originalName',
          ),
          mimeType: BuiltValueNullFieldError.checkNotNull(
            mimeType,
            r'RunContextAttachment',
            'mimeType',
          ),
          sizeBytes: BuiltValueNullFieldError.checkNotNull(
            sizeBytes,
            r'RunContextAttachment',
            'sizeBytes',
          ),
          scanStatus: BuiltValueNullFieldError.checkNotNull(
            scanStatus,
            r'RunContextAttachment',
            'scanStatus',
          ),
          transcript: transcript,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
