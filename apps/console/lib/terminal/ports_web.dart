// SPDX-License-Identifier: Apache-2.0

/// The terminal seams of the web build through package:web: the browser `WebSocket` of a
/// stream, IndexedDB of the origin for the pins and `window.open` for a confirmed link.
library;

import 'dart:async';
import 'dart:js_interop';
import 'dart:typed_data';

import 'package:web/web.dart' as web;

import 'ports.dart';

/// The terminal ports of the page the console runs in.
ConsoleTerminalPorts terminalPorts() => ConsoleTerminalPorts(
  connect: BrowserTerminalSocket.connect,
  pins: IndexedDbPinDatabase(web.window.indexedDB),
  openLink: (target) =>
      // No opener and no referrer: the page behind the link gets no handle on the console.
      web.window.open(target.toString(), '_blank', 'noopener,noreferrer'),
);

/// [TerminalSocket] over the browser's `WebSocket` with binary type `arraybuffer`.
///
/// The browser sends the console cookie with the upgrade and the page's `Origin`, which is how
/// the server authenticates the stream (08 7.5); nothing here adds a credential.
final class BrowserTerminalSocket implements TerminalSocket {
  BrowserTerminalSocket._(this._socket);

  final web.WebSocket _socket;
  final StreamController<Object> _messages = StreamController<Object>();

  @override
  int? closeCode;

  @override
  String? closeReason;

  /// Opens a WebSocket to [url] and completes once it is open; throws when it closes or fails
  /// before that.
  static Future<TerminalSocket> connect(Uri url) {
    final socket = web.WebSocket(url.toString())..binaryType = 'arraybuffer';
    final channel = BrowserTerminalSocket._(socket);
    final opened = Completer<TerminalSocket>();
    socket.onopen = ((web.Event _) {
      if (!opened.isCompleted) opened.complete(channel);
    }).toJS;
    socket.onerror = ((web.Event _) {
      // The browser tells the page nothing about the cause (by design of the WebSocket API).
      if (!opened.isCompleted) {
        opened.completeError(StateError('terminal stream did not open'));
      }
    }).toJS;
    socket.onmessage = ((web.MessageEvent event) {
      final data = event.data;
      if (data.isA<JSString>()) {
        channel._messages.add((data as JSString).toDart);
      } else if (data.isA<JSArrayBuffer>()) {
        channel._messages.add((data as JSArrayBuffer).toDart.asUint8List());
      }
    }).toJS;
    socket.onclose = ((web.CloseEvent event) {
      channel.closeCode = event.code;
      channel.closeReason = event.reason.isEmpty ? null : event.reason;
      if (!opened.isCompleted) {
        opened.completeError(StateError('terminal stream closed before open'));
      }
      unawaited(channel._messages.close());
    }).toJS;
    return opened.future;
  }

  @override
  Stream<Object> get messages => _messages.stream;

  @override
  void sendText(String text) => _socket.send(text.toJS);

  @override
  void sendBinary(Uint8List bytes) => _socket.send(bytes.toJS);

  @override
  Future<void> close([int code = 1000]) async {
    _socket.close(code);
    if (!_messages.isClosed) await _messages.close();
  }
}

/// [PinDatabase] over IndexedDB of the origin: database [pinDatabaseName], object store
/// [pinStoreName], one record per host keyed by its ID.
///
/// Every operation opens the database and closes it again, so no connection of this tab stays
/// open and the deletion at logout in another tab is never blocked by it.
final class IndexedDbPinDatabase implements PinDatabase {
  /// Creates the database over the browser's [factory] (`window.indexedDB`).
  IndexedDbPinDatabase(this.factory);

  /// The IndexedDB factory of the window.
  final web.IDBFactory factory;

  /// The schema version; version 1 creates the store.
  static const int _version = 1;

  @override
  Future<Map<String, Object?>?> read(String hostId) async {
    final db = await _open();
    try {
      final store = db
          .transaction(pinStoreName.toJS, 'readonly')
          .objectStore(pinStoreName);
      final value = await _result(store.get(hostId.toJS));
      final plain = value.dartify();
      if (plain is! Map) return null;
      return {for (final e in plain.entries) '${e.key}': e.value};
    } finally {
      db.close();
    }
  }

  @override
  Future<void> write(String hostId, Map<String, Object?> record) async {
    final db = await _open();
    try {
      final tx = db.transaction(pinStoreName.toJS, 'readwrite');
      final done = _complete(tx);
      tx.objectStore(pinStoreName).put(record.jsify(), hostId.toJS);
      await done;
    } finally {
      db.close();
    }
  }

  @override
  Future<void> delete() async {
    final request = factory.deleteDatabase(pinDatabaseName);
    final done = Completer<void>();
    request.onsuccess = ((web.Event _) => done.complete()).toJS;
    request.onerror = ((web.Event _) {
      if (!done.isCompleted) done.completeError(StateError('pins not deleted'));
    }).toJS;
    // A connection of another tab holds the deletion until it closes; the stores close theirs
    // after every operation, so the deletion goes through right after.
    await done.future;
  }

  Future<web.IDBDatabase> _open() {
    final request = factory.open(pinDatabaseName, _version);
    final done = Completer<web.IDBDatabase>();
    request.onupgradeneeded = ((web.Event _) {
      final db = request.result as web.IDBDatabase;
      if (!db.objectStoreNames.contains(pinStoreName)) {
        db.createObjectStore(pinStoreName);
      }
    }).toJS;
    request.onsuccess = ((web.Event _) {
      done.complete(request.result as web.IDBDatabase);
    }).toJS;
    request.onerror = ((web.Event _) {
      if (!done.isCompleted) done.completeError(StateError('pins not opened'));
    }).toJS;
    return done.future;
  }

  static Future<JSAny?> _result(web.IDBRequest request) {
    final done = Completer<JSAny?>();
    request.onsuccess = ((web.Event _) => done.complete(request.result)).toJS;
    request.onerror = ((web.Event _) {
      if (!done.isCompleted) done.completeError(StateError('pins not read'));
    }).toJS;
    return done.future;
  }

  static Future<void> _complete(web.IDBTransaction tx) {
    final done = Completer<void>();
    tx.oncomplete = ((web.Event _) => done.complete()).toJS;
    tx.onerror = ((web.Event _) {
      if (!done.isCompleted) done.completeError(StateError('pin not stored'));
    }).toJS;
    return done.future;
  }
}
