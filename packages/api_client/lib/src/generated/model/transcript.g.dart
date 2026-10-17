// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'transcript.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Transcript extends Transcript {
  @override
  final String fileId;
  @override
  final TranscriptStatus status;
  @override
  final String? text;
  @override
  final String? language;
  @override
  final TranscriptSource? source_;

  factory _$Transcript([void Function(TranscriptBuilder)? updates]) =>
      (TranscriptBuilder()..update(updates))._build();

  _$Transcript._({
    required this.fileId,
    required this.status,
    this.text,
    this.language,
    this.source_,
  }) : super._();
  @override
  Transcript rebuild(void Function(TranscriptBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TranscriptBuilder toBuilder() => TranscriptBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Transcript &&
        fileId == other.fileId &&
        status == other.status &&
        text == other.text &&
        language == other.language &&
        source_ == other.source_;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, fileId.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, text.hashCode);
    _$hash = $jc(_$hash, language.hashCode);
    _$hash = $jc(_$hash, source_.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Transcript')
          ..add('fileId', fileId)
          ..add('status', status)
          ..add('text', text)
          ..add('language', language)
          ..add('source_', source_))
        .toString();
  }
}

class TranscriptBuilder implements Builder<Transcript, TranscriptBuilder> {
  _$Transcript? _$v;

  String? _fileId;
  String? get fileId => _$this._fileId;
  set fileId(String? fileId) => _$this._fileId = fileId;

  TranscriptStatus? _status;
  TranscriptStatus? get status => _$this._status;
  set status(TranscriptStatus? status) => _$this._status = status;

  String? _text;
  String? get text => _$this._text;
  set text(String? text) => _$this._text = text;

  String? _language;
  String? get language => _$this._language;
  set language(String? language) => _$this._language = language;

  TranscriptSource? _source_;
  TranscriptSource? get source_ => _$this._source_;
  set source_(TranscriptSource? source_) => _$this._source_ = source_;

  TranscriptBuilder() {
    Transcript._defaults(this);
  }

  TranscriptBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _fileId = $v.fileId;
      _status = $v.status;
      _text = $v.text;
      _language = $v.language;
      _source_ = $v.source_;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Transcript other) {
    _$v = other as _$Transcript;
  }

  @override
  void update(void Function(TranscriptBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Transcript build() => _build();

  _$Transcript _build() {
    final _$result =
        _$v ??
        _$Transcript._(
          fileId: BuiltValueNullFieldError.checkNotNull(
            fileId,
            r'Transcript',
            'fileId',
          ),
          status: BuiltValueNullFieldError.checkNotNull(
            status,
            r'Transcript',
            'status',
          ),
          text: text,
          language: language,
          source_: source_,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
