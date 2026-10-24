// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_pin_data.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportPinData extends ImportPinData {
  @override
  final String messageOriginKey;

  factory _$ImportPinData([void Function(ImportPinDataBuilder)? updates]) =>
      (ImportPinDataBuilder()..update(updates))._build();

  _$ImportPinData._({required this.messageOriginKey}) : super._();
  @override
  ImportPinData rebuild(void Function(ImportPinDataBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportPinDataBuilder toBuilder() => ImportPinDataBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportPinData && messageOriginKey == other.messageOriginKey;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, messageOriginKey.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ImportPinData',
    )..add('messageOriginKey', messageOriginKey)).toString();
  }
}

class ImportPinDataBuilder
    implements Builder<ImportPinData, ImportPinDataBuilder> {
  _$ImportPinData? _$v;

  String? _messageOriginKey;
  String? get messageOriginKey => _$this._messageOriginKey;
  set messageOriginKey(String? messageOriginKey) =>
      _$this._messageOriginKey = messageOriginKey;

  ImportPinDataBuilder() {
    ImportPinData._defaults(this);
  }

  ImportPinDataBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _messageOriginKey = $v.messageOriginKey;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportPinData other) {
    _$v = other as _$ImportPinData;
  }

  @override
  void update(void Function(ImportPinDataBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportPinData build() => _build();

  _$ImportPinData _build() {
    final _$result =
        _$v ??
        _$ImportPinData._(
          messageOriginKey: BuiltValueNullFieldError.checkNotNull(
            messageOriginKey,
            r'ImportPinData',
            'messageOriginKey',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
