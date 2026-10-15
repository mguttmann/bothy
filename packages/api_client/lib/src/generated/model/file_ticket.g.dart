// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'file_ticket.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$FileTicket extends FileTicket {
  @override
  final String url;
  @override
  final DateTime expiresAt;

  factory _$FileTicket([void Function(FileTicketBuilder)? updates]) =>
      (FileTicketBuilder()..update(updates))._build();

  _$FileTicket._({required this.url, required this.expiresAt}) : super._();
  @override
  FileTicket rebuild(void Function(FileTicketBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  FileTicketBuilder toBuilder() => FileTicketBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is FileTicket &&
        url == other.url &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, url.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'FileTicket')
          ..add('url', url)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class FileTicketBuilder implements Builder<FileTicket, FileTicketBuilder> {
  _$FileTicket? _$v;

  String? _url;
  String? get url => _$this._url;
  set url(String? url) => _$this._url = url;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  FileTicketBuilder() {
    FileTicket._defaults(this);
  }

  FileTicketBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _url = $v.url;
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(FileTicket other) {
    _$v = other as _$FileTicket;
  }

  @override
  void update(void Function(FileTicketBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  FileTicket build() => _build();

  _$FileTicket _build() {
    final _$result =
        _$v ??
        _$FileTicket._(
          url: BuiltValueNullFieldError.checkNotNull(url, r'FileTicket', 'url'),
          expiresAt: BuiltValueNullFieldError.checkNotNull(
            expiresAt,
            r'FileTicket',
            'expiresAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
