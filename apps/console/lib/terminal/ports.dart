// SPDX-License-Identifier: Apache-2.0

/// The browser seams of the console's terminal (brief M6-18 Design note 3): the WebSocket of
/// one stream, the IndexedDB of the pins and a new tab for a confirmed link. The web build
/// implements them with package:web (`ports_web.dart`); every other platform, the test VM
/// included, gets `ports_stub.dart`, which opens no socket and keeps the pins in memory. The
/// ports of `terminal_ui` in this folder sit on top of these seams, so their tests run on the
/// VM with fakes.
library;

import 'dart:typed_data';

import 'ports_stub.dart' if (dart.library.js_interop) 'ports_web.dart' as impl;

/// The name of the IndexedDB database of the pins; one per origin, so one per console.
const String pinDatabaseName = 'bothy.terminal.pins';

/// The object store of the pins in [pinDatabaseName]: one record per host, keyed by host ID.
const String pinStoreName = 'host_key_pins';

/// One open WebSocket of a terminal stream, as the browser delivers it.
///
/// The seam is this small on purpose: the order of the protocol (text until `identify`, then
/// binary) is checked above it in `ConsoleTerminalTransport`, so a fake socket proves it.
abstract interface class TerminalSocket {
  /// Every received message: a [String] for a text message, a [Uint8List] for a binary one.
  /// The stream ends when the connection closes.
  Stream<Object> get messages;

  /// Sends a text message.
  void sendText(String text);

  /// Sends a binary message.
  void sendBinary(Uint8List bytes);

  /// Closes the connection with [code] (a normal close by default).
  Future<void> close([int code = 1000]);

  /// The close code the server sent, null while open or without one.
  int? get closeCode;

  /// The close reason the server sent, null without one.
  String? get closeReason;
}

/// Opens a WebSocket to [url]; completes when it is open and throws when it cannot open.
typedef TerminalSocketConnector = Future<TerminalSocket> Function(Uri url);

/// The few operations of IndexedDB the pin store needs: one record per host in the object
/// store [pinStoreName] of the database [pinDatabaseName], and deleting the whole database.
///
/// A record is a map of plain values (strings and integers), so the browser stores it with the
/// structured clone and the store never hands IndexedDB an object of its own.
abstract interface class PinDatabase {
  /// The record of [hostId], or null when there is none.
  Future<Map<String, Object?>?> read(String hostId);

  /// Stores [record] for [hostId], replacing an older one.
  Future<void> write(String hostId, Map<String, Object?> record);

  /// Deletes the whole database of the origin (logout, ADR 0070 item 3).
  Future<void> delete();
}

/// A [PinDatabase] in the memory of the tab: the console outside a browser, where nothing
/// outlives the page anyway.
final class MemoryPinDatabase implements PinDatabase {
  /// Creates an empty database.
  MemoryPinDatabase();

  final Map<String, Map<String, Object?>> _records = {};

  @override
  Future<Map<String, Object?>?> read(String hostId) async {
    final record = _records[hostId];
    return record == null ? null : Map.of(record);
  }

  @override
  Future<void> write(String hostId, Map<String, Object?> record) async =>
      _records[hostId] = Map.of(record);

  @override
  Future<void> delete() async => _records.clear();
}

/// The browser seams of the terminal, bundled for the console's composition root (`run.dart`
/// passes [ConsoleTerminalPorts.browser] to the app; tests pass fakes).
///
/// Example:
/// ```dart
/// runApp(ConsoleApp(environment: env, terminalPorts: ConsoleTerminalPorts.browser()));
/// ```
final class ConsoleTerminalPorts {
  /// Creates the ports from their parts.
  const ConsoleTerminalPorts({
    required this.connect,
    required this.pins,
    required this.openLink,
  });

  /// The ports of the page the console runs in (the web build), or inert ones on any other
  /// platform: no socket opens and the pins stay in memory.
  factory ConsoleTerminalPorts.browser() => impl.terminalPorts();

  /// Opens the WebSocket of a stream.
  final TerminalSocketConnector connect;

  /// The IndexedDB of the pins.
  final PinDatabase pins;

  /// Opens a link the person confirmed in a new tab, without access to the console's window.
  final void Function(Uri target) openLink;
}
