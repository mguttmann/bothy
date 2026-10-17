// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_progress_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportProgressPayload extends ImportProgressPayload {
  @override
  final ImportSource import_;

  factory _$ImportProgressPayload([
    void Function(ImportProgressPayloadBuilder)? updates,
  ]) => (ImportProgressPayloadBuilder()..update(updates))._build();

  _$ImportProgressPayload._({required this.import_}) : super._();
  @override
  ImportProgressPayload rebuild(
    void Function(ImportProgressPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ImportProgressPayloadBuilder toBuilder() =>
      ImportProgressPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportProgressPayload && import_ == other.import_;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, import_.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ImportProgressPayload',
    )..add('import_', import_)).toString();
  }
}

class ImportProgressPayloadBuilder
    implements Builder<ImportProgressPayload, ImportProgressPayloadBuilder> {
  _$ImportProgressPayload? _$v;

  ImportSourceBuilder? _import_;
  ImportSourceBuilder get import_ => _$this._import_ ??= ImportSourceBuilder();
  set import_(ImportSourceBuilder? import_) => _$this._import_ = import_;

  ImportProgressPayloadBuilder() {
    ImportProgressPayload._defaults(this);
  }

  ImportProgressPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _import_ = $v.import_.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportProgressPayload other) {
    _$v = other as _$ImportProgressPayload;
  }

  @override
  void update(void Function(ImportProgressPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportProgressPayload build() => _build();

  _$ImportProgressPayload _build() {
    _$ImportProgressPayload _$result;
    try {
      _$result = _$v ?? _$ImportProgressPayload._(import_: import_.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'import_';
        import_.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportProgressPayload',
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
