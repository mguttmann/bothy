// SPDX-License-Identifier: Apache-2.0

// Test helpers: the shared test data of services/host/testdata/noise and hex conversion.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:cryptography/cryptography.dart';

/// Returns services/host/testdata/noise, found by walking up from the package directory.
Directory testdataDir() {
  var dir = Directory.current.absolute;
  while (true) {
    final candidate = Directory('${dir.path}/services/host/testdata/noise');
    if (candidate.existsSync()) {
      return candidate;
    }
    final parent = dir.parent;
    if (parent.path == dir.path) {
      throw StateError(
        'no directory above ${Directory.current.path} contains services/host/testdata/noise',
      );
    }
    dir = parent;
  }
}

/// Decodes lowercase or uppercase hex.
Uint8List fromHex(String s) {
  if (s.length.isOdd) {
    throw FormatException('odd hex length ${s.length}');
  }
  final out = Uint8List(s.length ~/ 2);
  for (var i = 0; i < out.length; i++) {
    out[i] = int.parse(s.substring(2 * i, 2 * i + 2), radix: 16);
  }
  return out;
}

/// Encodes bytes as lowercase hex.
String toHex(List<int> b) =>
    b.map((x) => x.toRadixString(16).padLeft(2, '0')).join();

/// SHA-256 of the UTF-8 bytes of [label]: the deterministic test keys.
Future<Uint8List> sha256Of(String label) async =>
    Uint8List.fromList((await Sha256().hash(utf8.encode(label))).bytes);

/// The X25519 key pair of a 32-byte private key.
Future<SimpleKeyPair> keyPairOf(List<int> private) =>
    X25519().newKeyPairFromSeed(private);

/// The public key bytes of [kp].
Future<Uint8List> publicOf(SimpleKeyPair kp) async =>
    Uint8List.fromList((await kp.extractPublicKey()).bytes);

/// A short, non-secret description of bytes for failure messages: length and hash prefix.
Future<String> describe(List<int> b) async {
  final h = await Sha256().hash(b);
  return '${b.length} bytes, sha256 ${toHex(h.bytes.sublist(0, 4))}';
}

/// Fails with lengths and hash prefixes only, never the bytes themselves.
Future<void> expectSameBytes(String what, List<int> got, List<int> want) async {
  var same = got.length == want.length;
  for (var i = 0; same && i < got.length; i++) {
    same = got[i] == want[i];
  }
  if (!same) {
    throw StateError(
      '$what differs: ${await describe(got)}, want ${await describe(want)}',
    );
  }
}
