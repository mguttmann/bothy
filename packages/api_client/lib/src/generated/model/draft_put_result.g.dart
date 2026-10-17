// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_put_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DraftPutResult extends DraftPutResult {
  @override
  final Draft draft;
  @override
  final bool overwroteNewer;

  factory _$DraftPutResult([void Function(DraftPutResultBuilder)? updates]) =>
      (DraftPutResultBuilder()..update(updates))._build();

  _$DraftPutResult._({required this.draft, required this.overwroteNewer})
    : super._();
  @override
  DraftPutResult rebuild(void Function(DraftPutResultBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DraftPutResultBuilder toBuilder() => DraftPutResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DraftPutResult &&
        draft == other.draft &&
        overwroteNewer == other.overwroteNewer;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, draft.hashCode);
    _$hash = $jc(_$hash, overwroteNewer.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DraftPutResult')
          ..add('draft', draft)
          ..add('overwroteNewer', overwroteNewer))
        .toString();
  }
}

class DraftPutResultBuilder
    implements Builder<DraftPutResult, DraftPutResultBuilder> {
  _$DraftPutResult? _$v;

  DraftBuilder? _draft;
  DraftBuilder get draft => _$this._draft ??= DraftBuilder();
  set draft(DraftBuilder? draft) => _$this._draft = draft;

  bool? _overwroteNewer;
  bool? get overwroteNewer => _$this._overwroteNewer;
  set overwroteNewer(bool? overwroteNewer) =>
      _$this._overwroteNewer = overwroteNewer;

  DraftPutResultBuilder() {
    DraftPutResult._defaults(this);
  }

  DraftPutResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _draft = $v.draft.toBuilder();
      _overwroteNewer = $v.overwroteNewer;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DraftPutResult other) {
    _$v = other as _$DraftPutResult;
  }

  @override
  void update(void Function(DraftPutResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DraftPutResult build() => _build();

  _$DraftPutResult _build() {
    _$DraftPutResult _$result;
    try {
      _$result =
          _$v ??
          _$DraftPutResult._(
            draft: draft.build(),
            overwroteNewer: BuiltValueNullFieldError.checkNotNull(
              overwroteNewer,
              r'DraftPutResult',
              'overwroteNewer',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'draft';
        draft.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DraftPutResult',
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
