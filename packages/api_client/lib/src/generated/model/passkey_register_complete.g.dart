// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'passkey_register_complete.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PasskeyRegisterComplete extends PasskeyRegisterComplete {
  @override
  final String challengeId;
  @override
  final BuiltMap<String, JsonObject?> credential;
  @override
  final String name;

  factory _$PasskeyRegisterComplete([
    void Function(PasskeyRegisterCompleteBuilder)? updates,
  ]) => (PasskeyRegisterCompleteBuilder()..update(updates))._build();

  _$PasskeyRegisterComplete._({
    required this.challengeId,
    required this.credential,
    required this.name,
  }) : super._();
  @override
  PasskeyRegisterComplete rebuild(
    void Function(PasskeyRegisterCompleteBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PasskeyRegisterCompleteBuilder toBuilder() =>
      PasskeyRegisterCompleteBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PasskeyRegisterComplete &&
        challengeId == other.challengeId &&
        credential == other.credential &&
        name == other.name;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, challengeId.hashCode);
    _$hash = $jc(_$hash, credential.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PasskeyRegisterComplete')
          ..add('challengeId', challengeId)
          ..add('credential', credential)
          ..add('name', name))
        .toString();
  }
}

class PasskeyRegisterCompleteBuilder
    implements
        Builder<PasskeyRegisterComplete, PasskeyRegisterCompleteBuilder> {
  _$PasskeyRegisterComplete? _$v;

  String? _challengeId;
  String? get challengeId => _$this._challengeId;
  set challengeId(String? challengeId) => _$this._challengeId = challengeId;

  MapBuilder<String, JsonObject?>? _credential;
  MapBuilder<String, JsonObject?> get credential =>
      _$this._credential ??= MapBuilder<String, JsonObject?>();
  set credential(MapBuilder<String, JsonObject?>? credential) =>
      _$this._credential = credential;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  PasskeyRegisterCompleteBuilder() {
    PasskeyRegisterComplete._defaults(this);
  }

  PasskeyRegisterCompleteBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _challengeId = $v.challengeId;
      _credential = $v.credential.toBuilder();
      _name = $v.name;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PasskeyRegisterComplete other) {
    _$v = other as _$PasskeyRegisterComplete;
  }

  @override
  void update(void Function(PasskeyRegisterCompleteBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PasskeyRegisterComplete build() => _build();

  _$PasskeyRegisterComplete _build() {
    _$PasskeyRegisterComplete _$result;
    try {
      _$result =
          _$v ??
          _$PasskeyRegisterComplete._(
            challengeId: BuiltValueNullFieldError.checkNotNull(
              challengeId,
              r'PasskeyRegisterComplete',
              'challengeId',
            ),
            credential: credential.build(),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'PasskeyRegisterComplete',
              'name',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'credential';
        credential.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'PasskeyRegisterComplete',
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
