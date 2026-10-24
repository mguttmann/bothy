// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_thread_data.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportThreadData extends ImportThreadData {
  @override
  final String channelOriginKey;
  @override
  final String name;
  @override
  final DateTime createdAt;
  @override
  final DateTime lastActivityAt;
  @override
  final bool closed;
  @override
  final DateTime? closedAt;
  @override
  final String? starterMessageOriginKey;
  @override
  final BuiltList<String>? tagOriginKeys;

  factory _$ImportThreadData([
    void Function(ImportThreadDataBuilder)? updates,
  ]) => (ImportThreadDataBuilder()..update(updates))._build();

  _$ImportThreadData._({
    required this.channelOriginKey,
    required this.name,
    required this.createdAt,
    required this.lastActivityAt,
    required this.closed,
    this.closedAt,
    this.starterMessageOriginKey,
    this.tagOriginKeys,
  }) : super._();
  @override
  ImportThreadData rebuild(void Function(ImportThreadDataBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportThreadDataBuilder toBuilder() =>
      ImportThreadDataBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportThreadData &&
        channelOriginKey == other.channelOriginKey &&
        name == other.name &&
        createdAt == other.createdAt &&
        lastActivityAt == other.lastActivityAt &&
        closed == other.closed &&
        closedAt == other.closedAt &&
        starterMessageOriginKey == other.starterMessageOriginKey &&
        tagOriginKeys == other.tagOriginKeys;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelOriginKey.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, lastActivityAt.hashCode);
    _$hash = $jc(_$hash, closed.hashCode);
    _$hash = $jc(_$hash, closedAt.hashCode);
    _$hash = $jc(_$hash, starterMessageOriginKey.hashCode);
    _$hash = $jc(_$hash, tagOriginKeys.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportThreadData')
          ..add('channelOriginKey', channelOriginKey)
          ..add('name', name)
          ..add('createdAt', createdAt)
          ..add('lastActivityAt', lastActivityAt)
          ..add('closed', closed)
          ..add('closedAt', closedAt)
          ..add('starterMessageOriginKey', starterMessageOriginKey)
          ..add('tagOriginKeys', tagOriginKeys))
        .toString();
  }
}

class ImportThreadDataBuilder
    implements Builder<ImportThreadData, ImportThreadDataBuilder> {
  _$ImportThreadData? _$v;

  String? _channelOriginKey;
  String? get channelOriginKey => _$this._channelOriginKey;
  set channelOriginKey(String? channelOriginKey) =>
      _$this._channelOriginKey = channelOriginKey;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _lastActivityAt;
  DateTime? get lastActivityAt => _$this._lastActivityAt;
  set lastActivityAt(DateTime? lastActivityAt) =>
      _$this._lastActivityAt = lastActivityAt;

  bool? _closed;
  bool? get closed => _$this._closed;
  set closed(bool? closed) => _$this._closed = closed;

  DateTime? _closedAt;
  DateTime? get closedAt => _$this._closedAt;
  set closedAt(DateTime? closedAt) => _$this._closedAt = closedAt;

  String? _starterMessageOriginKey;
  String? get starterMessageOriginKey => _$this._starterMessageOriginKey;
  set starterMessageOriginKey(String? starterMessageOriginKey) =>
      _$this._starterMessageOriginKey = starterMessageOriginKey;

  ListBuilder<String>? _tagOriginKeys;
  ListBuilder<String> get tagOriginKeys =>
      _$this._tagOriginKeys ??= ListBuilder<String>();
  set tagOriginKeys(ListBuilder<String>? tagOriginKeys) =>
      _$this._tagOriginKeys = tagOriginKeys;

  ImportThreadDataBuilder() {
    ImportThreadData._defaults(this);
  }

  ImportThreadDataBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelOriginKey = $v.channelOriginKey;
      _name = $v.name;
      _createdAt = $v.createdAt;
      _lastActivityAt = $v.lastActivityAt;
      _closed = $v.closed;
      _closedAt = $v.closedAt;
      _starterMessageOriginKey = $v.starterMessageOriginKey;
      _tagOriginKeys = $v.tagOriginKeys?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportThreadData other) {
    _$v = other as _$ImportThreadData;
  }

  @override
  void update(void Function(ImportThreadDataBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportThreadData build() => _build();

  _$ImportThreadData _build() {
    _$ImportThreadData _$result;
    try {
      _$result =
          _$v ??
          _$ImportThreadData._(
            channelOriginKey: BuiltValueNullFieldError.checkNotNull(
              channelOriginKey,
              r'ImportThreadData',
              'channelOriginKey',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'ImportThreadData',
              'name',
            ),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'ImportThreadData',
              'createdAt',
            ),
            lastActivityAt: BuiltValueNullFieldError.checkNotNull(
              lastActivityAt,
              r'ImportThreadData',
              'lastActivityAt',
            ),
            closed: BuiltValueNullFieldError.checkNotNull(
              closed,
              r'ImportThreadData',
              'closed',
            ),
            closedAt: closedAt,
            starterMessageOriginKey: starterMessageOriginKey,
            tagOriginKeys: _tagOriginKeys?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'tagOriginKeys';
        _tagOriginKeys?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ImportThreadData',
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
