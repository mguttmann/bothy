// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DraftPayload extends DraftPayload {
  @override
  final Draft draft;

  factory _$DraftPayload([void Function(DraftPayloadBuilder)? updates]) =>
      (DraftPayloadBuilder()..update(updates))._build();

  _$DraftPayload._({required this.draft}) : super._();
  @override
  DraftPayload rebuild(void Function(DraftPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DraftPayloadBuilder toBuilder() => DraftPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DraftPayload && draft == other.draft;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, draft.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'DraftPayload',
    )..add('draft', draft)).toString();
  }
}

class DraftPayloadBuilder
    implements Builder<DraftPayload, DraftPayloadBuilder> {
  _$DraftPayload? _$v;

  DraftBuilder? _draft;
  DraftBuilder get draft => _$this._draft ??= DraftBuilder();
  set draft(DraftBuilder? draft) => _$this._draft = draft;

  DraftPayloadBuilder() {
    DraftPayload._defaults(this);
  }

  DraftPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _draft = $v.draft.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DraftPayload other) {
    _$v = other as _$DraftPayload;
  }

  @override
  void update(void Function(DraftPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DraftPayload build() => _build();

  _$DraftPayload _build() {
    _$DraftPayload _$result;
    try {
      _$result = _$v ?? _$DraftPayload._(draft: draft.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'draft';
        draft.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DraftPayload',
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
