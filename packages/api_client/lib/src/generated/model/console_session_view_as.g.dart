// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'console_session_view_as.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ConsoleSessionViewAs extends ConsoleSessionViewAs {
  @override
  final String userId;
  @override
  final String displayName;
  @override
  final DateTime startedAt;
  @override
  final DateTime endsAt;

  factory _$ConsoleSessionViewAs([
    void Function(ConsoleSessionViewAsBuilder)? updates,
  ]) => (ConsoleSessionViewAsBuilder()..update(updates))._build();

  _$ConsoleSessionViewAs._({
    required this.userId,
    required this.displayName,
    required this.startedAt,
    required this.endsAt,
  }) : super._();
  @override
  ConsoleSessionViewAs rebuild(
    void Function(ConsoleSessionViewAsBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ConsoleSessionViewAsBuilder toBuilder() =>
      ConsoleSessionViewAsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ConsoleSessionViewAs &&
        userId == other.userId &&
        displayName == other.displayName &&
        startedAt == other.startedAt &&
        endsAt == other.endsAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, endsAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ConsoleSessionViewAs')
          ..add('userId', userId)
          ..add('displayName', displayName)
          ..add('startedAt', startedAt)
          ..add('endsAt', endsAt))
        .toString();
  }
}

class ConsoleSessionViewAsBuilder
    implements Builder<ConsoleSessionViewAs, ConsoleSessionViewAsBuilder> {
  _$ConsoleSessionViewAs? _$v;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _endsAt;
  DateTime? get endsAt => _$this._endsAt;
  set endsAt(DateTime? endsAt) => _$this._endsAt = endsAt;

  ConsoleSessionViewAsBuilder() {
    ConsoleSessionViewAs._defaults(this);
  }

  ConsoleSessionViewAsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _userId = $v.userId;
      _displayName = $v.displayName;
      _startedAt = $v.startedAt;
      _endsAt = $v.endsAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ConsoleSessionViewAs other) {
    _$v = other as _$ConsoleSessionViewAs;
  }

  @override
  void update(void Function(ConsoleSessionViewAsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ConsoleSessionViewAs build() => _build();

  _$ConsoleSessionViewAs _build() {
    final _$result =
        _$v ??
        _$ConsoleSessionViewAs._(
          userId: BuiltValueNullFieldError.checkNotNull(
            userId,
            r'ConsoleSessionViewAs',
            'userId',
          ),
          displayName: BuiltValueNullFieldError.checkNotNull(
            displayName,
            r'ConsoleSessionViewAs',
            'displayName',
          ),
          startedAt: BuiltValueNullFieldError.checkNotNull(
            startedAt,
            r'ConsoleSessionViewAs',
            'startedAt',
          ),
          endsAt: BuiltValueNullFieldError.checkNotNull(
            endsAt,
            r'ConsoleSessionViewAs',
            'endsAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
