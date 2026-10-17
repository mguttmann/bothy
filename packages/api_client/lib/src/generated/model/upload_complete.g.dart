// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'upload_complete.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UploadComplete extends UploadComplete {
  @override
  final BuiltList<UploadPartResult> parts;

  factory _$UploadComplete([void Function(UploadCompleteBuilder)? updates]) =>
      (UploadCompleteBuilder()..update(updates))._build();

  _$UploadComplete._({required this.parts}) : super._();
  @override
  UploadComplete rebuild(void Function(UploadCompleteBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UploadCompleteBuilder toBuilder() => UploadCompleteBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UploadComplete && parts == other.parts;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, parts.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'UploadComplete',
    )..add('parts', parts)).toString();
  }
}

class UploadCompleteBuilder
    implements Builder<UploadComplete, UploadCompleteBuilder> {
  _$UploadComplete? _$v;

  ListBuilder<UploadPartResult>? _parts;
  ListBuilder<UploadPartResult> get parts =>
      _$this._parts ??= ListBuilder<UploadPartResult>();
  set parts(ListBuilder<UploadPartResult>? parts) => _$this._parts = parts;

  UploadCompleteBuilder() {
    UploadComplete._defaults(this);
  }

  UploadCompleteBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _parts = $v.parts.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UploadComplete other) {
    _$v = other as _$UploadComplete;
  }

  @override
  void update(void Function(UploadCompleteBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UploadComplete build() => _build();

  _$UploadComplete _build() {
    _$UploadComplete _$result;
    try {
      _$result = _$v ?? _$UploadComplete._(parts: parts.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'parts';
        parts.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'UploadComplete',
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
