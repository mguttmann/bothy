// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'transcript_submit.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TranscriptSubmit extends TranscriptSubmit {
  @override
  final String text;
  @override
  final String? language;

  factory _$TranscriptSubmit([
    void Function(TranscriptSubmitBuilder)? updates,
  ]) => (TranscriptSubmitBuilder()..update(updates))._build();

  _$TranscriptSubmit._({required this.text, this.language}) : super._();
  @override
  TranscriptSubmit rebuild(void Function(TranscriptSubmitBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TranscriptSubmitBuilder toBuilder() =>
      TranscriptSubmitBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TranscriptSubmit &&
        text == other.text &&
        language == other.language;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, text.hashCode);
    _$hash = $jc(_$hash, language.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TranscriptSubmit')
          ..add('text', text)
          ..add('language', language))
        .toString();
  }
}

class TranscriptSubmitBuilder
    implements Builder<TranscriptSubmit, TranscriptSubmitBuilder> {
  _$TranscriptSubmit? _$v;

  String? _text;
  String? get text => _$this._text;
  set text(String? text) => _$this._text = text;

  String? _language;
  String? get language => _$this._language;
  set language(String? language) => _$this._language = language;

  TranscriptSubmitBuilder() {
    TranscriptSubmit._defaults(this);
  }

  TranscriptSubmitBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _text = $v.text;
      _language = $v.language;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TranscriptSubmit other) {
    _$v = other as _$TranscriptSubmit;
  }

  @override
  void update(void Function(TranscriptSubmitBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TranscriptSubmit build() => _build();

  _$TranscriptSubmit _build() {
    final _$result =
        _$v ??
        _$TranscriptSubmit._(
          text: BuiltValueNullFieldError.checkNotNull(
            text,
            r'TranscriptSubmit',
            'text',
          ),
          language: language,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
