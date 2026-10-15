// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'manifest_validation.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ManifestValidation extends ManifestValidation {
  @override
  final bool valid;
  @override
  final BuiltList<FieldError> errors;

  factory _$ManifestValidation([
    void Function(ManifestValidationBuilder)? updates,
  ]) => (ManifestValidationBuilder()..update(updates))._build();

  _$ManifestValidation._({required this.valid, required this.errors})
    : super._();
  @override
  ManifestValidation rebuild(
    void Function(ManifestValidationBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ManifestValidationBuilder toBuilder() =>
      ManifestValidationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ManifestValidation &&
        valid == other.valid &&
        errors == other.errors;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, valid.hashCode);
    _$hash = $jc(_$hash, errors.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ManifestValidation')
          ..add('valid', valid)
          ..add('errors', errors))
        .toString();
  }
}

class ManifestValidationBuilder
    implements Builder<ManifestValidation, ManifestValidationBuilder> {
  _$ManifestValidation? _$v;

  bool? _valid;
  bool? get valid => _$this._valid;
  set valid(bool? valid) => _$this._valid = valid;

  ListBuilder<FieldError>? _errors;
  ListBuilder<FieldError> get errors =>
      _$this._errors ??= ListBuilder<FieldError>();
  set errors(ListBuilder<FieldError>? errors) => _$this._errors = errors;

  ManifestValidationBuilder() {
    ManifestValidation._defaults(this);
  }

  ManifestValidationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _valid = $v.valid;
      _errors = $v.errors.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ManifestValidation other) {
    _$v = other as _$ManifestValidation;
  }

  @override
  void update(void Function(ManifestValidationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ManifestValidation build() => _build();

  _$ManifestValidation _build() {
    _$ManifestValidation _$result;
    try {
      _$result =
          _$v ??
          _$ManifestValidation._(
            valid: BuiltValueNullFieldError.checkNotNull(
              valid,
              r'ManifestValidation',
              'valid',
            ),
            errors: errors.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'errors';
        errors.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ManifestValidation',
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
