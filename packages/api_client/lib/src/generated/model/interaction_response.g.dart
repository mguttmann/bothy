// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'interaction_response.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InteractionResponse extends InteractionResponse {
  @override
  final String? text;

  factory _$InteractionResponse([
    void Function(InteractionResponseBuilder)? updates,
  ]) => (InteractionResponseBuilder()..update(updates))._build();

  _$InteractionResponse._({this.text}) : super._();
  @override
  InteractionResponse rebuild(
    void Function(InteractionResponseBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InteractionResponseBuilder toBuilder() =>
      InteractionResponseBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InteractionResponse && text == other.text;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, text.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'InteractionResponse',
    )..add('text', text)).toString();
  }
}

class InteractionResponseBuilder
    implements Builder<InteractionResponse, InteractionResponseBuilder> {
  _$InteractionResponse? _$v;

  String? _text;
  String? get text => _$this._text;
  set text(String? text) => _$this._text = text;

  InteractionResponseBuilder() {
    InteractionResponse._defaults(this);
  }

  InteractionResponseBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _text = $v.text;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InteractionResponse other) {
    _$v = other as _$InteractionResponse;
  }

  @override
  void update(void Function(InteractionResponseBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InteractionResponse build() => _build();

  _$InteractionResponse _build() {
    final _$result = _$v ?? _$InteractionResponse._(text: text);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
