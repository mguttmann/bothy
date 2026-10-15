// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'instance_limits.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InstanceLimits extends InstanceLimits {
  @override
  final int messageMaxContentBytes;
  @override
  final int messageMaxAttachments;
  @override
  final int fileMaxUploadBytes;
  @override
  final int threadTitleMaxChars;
  @override
  final int voiceMaxDurationMs;

  factory _$InstanceLimits([void Function(InstanceLimitsBuilder)? updates]) =>
      (InstanceLimitsBuilder()..update(updates))._build();

  _$InstanceLimits._({
    required this.messageMaxContentBytes,
    required this.messageMaxAttachments,
    required this.fileMaxUploadBytes,
    required this.threadTitleMaxChars,
    required this.voiceMaxDurationMs,
  }) : super._();
  @override
  InstanceLimits rebuild(void Function(InstanceLimitsBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  InstanceLimitsBuilder toBuilder() => InstanceLimitsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InstanceLimits &&
        messageMaxContentBytes == other.messageMaxContentBytes &&
        messageMaxAttachments == other.messageMaxAttachments &&
        fileMaxUploadBytes == other.fileMaxUploadBytes &&
        threadTitleMaxChars == other.threadTitleMaxChars &&
        voiceMaxDurationMs == other.voiceMaxDurationMs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, messageMaxContentBytes.hashCode);
    _$hash = $jc(_$hash, messageMaxAttachments.hashCode);
    _$hash = $jc(_$hash, fileMaxUploadBytes.hashCode);
    _$hash = $jc(_$hash, threadTitleMaxChars.hashCode);
    _$hash = $jc(_$hash, voiceMaxDurationMs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'InstanceLimits')
          ..add('messageMaxContentBytes', messageMaxContentBytes)
          ..add('messageMaxAttachments', messageMaxAttachments)
          ..add('fileMaxUploadBytes', fileMaxUploadBytes)
          ..add('threadTitleMaxChars', threadTitleMaxChars)
          ..add('voiceMaxDurationMs', voiceMaxDurationMs))
        .toString();
  }
}

class InstanceLimitsBuilder
    implements Builder<InstanceLimits, InstanceLimitsBuilder> {
  _$InstanceLimits? _$v;

  int? _messageMaxContentBytes;
  int? get messageMaxContentBytes => _$this._messageMaxContentBytes;
  set messageMaxContentBytes(int? messageMaxContentBytes) =>
      _$this._messageMaxContentBytes = messageMaxContentBytes;

  int? _messageMaxAttachments;
  int? get messageMaxAttachments => _$this._messageMaxAttachments;
  set messageMaxAttachments(int? messageMaxAttachments) =>
      _$this._messageMaxAttachments = messageMaxAttachments;

  int? _fileMaxUploadBytes;
  int? get fileMaxUploadBytes => _$this._fileMaxUploadBytes;
  set fileMaxUploadBytes(int? fileMaxUploadBytes) =>
      _$this._fileMaxUploadBytes = fileMaxUploadBytes;

  int? _threadTitleMaxChars;
  int? get threadTitleMaxChars => _$this._threadTitleMaxChars;
  set threadTitleMaxChars(int? threadTitleMaxChars) =>
      _$this._threadTitleMaxChars = threadTitleMaxChars;

  int? _voiceMaxDurationMs;
  int? get voiceMaxDurationMs => _$this._voiceMaxDurationMs;
  set voiceMaxDurationMs(int? voiceMaxDurationMs) =>
      _$this._voiceMaxDurationMs = voiceMaxDurationMs;

  InstanceLimitsBuilder() {
    InstanceLimits._defaults(this);
  }

  InstanceLimitsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _messageMaxContentBytes = $v.messageMaxContentBytes;
      _messageMaxAttachments = $v.messageMaxAttachments;
      _fileMaxUploadBytes = $v.fileMaxUploadBytes;
      _threadTitleMaxChars = $v.threadTitleMaxChars;
      _voiceMaxDurationMs = $v.voiceMaxDurationMs;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InstanceLimits other) {
    _$v = other as _$InstanceLimits;
  }

  @override
  void update(void Function(InstanceLimitsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InstanceLimits build() => _build();

  _$InstanceLimits _build() {
    final _$result =
        _$v ??
        _$InstanceLimits._(
          messageMaxContentBytes: BuiltValueNullFieldError.checkNotNull(
            messageMaxContentBytes,
            r'InstanceLimits',
            'messageMaxContentBytes',
          ),
          messageMaxAttachments: BuiltValueNullFieldError.checkNotNull(
            messageMaxAttachments,
            r'InstanceLimits',
            'messageMaxAttachments',
          ),
          fileMaxUploadBytes: BuiltValueNullFieldError.checkNotNull(
            fileMaxUploadBytes,
            r'InstanceLimits',
            'fileMaxUploadBytes',
          ),
          threadTitleMaxChars: BuiltValueNullFieldError.checkNotNull(
            threadTitleMaxChars,
            r'InstanceLimits',
            'threadTitleMaxChars',
          ),
          voiceMaxDurationMs: BuiltValueNullFieldError.checkNotNull(
            voiceMaxDurationMs,
            r'InstanceLimits',
            'voiceMaxDurationMs',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
