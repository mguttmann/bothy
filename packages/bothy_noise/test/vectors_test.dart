// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:bothy_noise/src/handshake.dart'
    show startInitiatorWithPrologue, startResponderWithPrologue;
import 'package:test/test.dart';

import 'support.dart';

/// The protocol of a vector entry by the loader rules (brief M2-38, Design note 10).
String protocolOf(Map<String, Object?> e) {
  final name = e['protocol_name'] ?? e['name'];
  if (name is String) return name;
  return 'Noise_${e['pattern']}_${e['dh']}_${e['cipher']}_${e['hash']}';
}

bool nonEmpty(Object? v) =>
    (v is String && v.isNotEmpty) || (v is List<Object?> && v.isNotEmpty);

bool selected(Map<String, Object?> e) =>
    protocolOf(e) == noiseProtocolName &&
    !nonEmpty(e['init_psk']) &&
    !nonEmpty(e['resp_psk']) &&
    !nonEmpty(e['init_psks']) &&
    !nonEmpty(e['resp_psks']) &&
    e['fallback'] != true;

List<Map<String, Object?>> loadEntries(String file) {
  final gz = File('${testdataDir().path}/vectors/$file').readAsBytesSync();
  final data =
      jsonDecode(utf8.decode(GZipCodec().decode(gz))) as Map<String, Object?>;
  return (data['vectors']! as List<Object?>).cast<Map<String, Object?>>();
}

Uint8List hexField(Map<String, Object?> e, String key) =>
    fromHex((e[key] as String?) ?? '');

/// Replays one entry and throws on the first mismatch.
Future<void> replay(Map<String, Object?> e) async {
  final messages = (e['messages']! as List<Object?>)
      .cast<Map<String, Object?>>();
  final initPrologue = hexField(e, 'init_prologue');
  final respPrologue = e.containsKey('resp_prologue')
      ? hexField(e, 'resp_prologue')
      : initPrologue;
  final init = await startInitiatorWithPrologue(
    hostStaticPublicKey: hexField(e, 'init_remote_static'),
    prologue: initPrologue,
    ephemeral: await keyPairOf(hexField(e, 'init_ephemeral')),
  );
  final resp = await startResponderWithPrologue(
    staticKeyPair: await keyPairOf(hexField(e, 'resp_static')),
    prologue: respPrologue,
    ephemeral: await keyPairOf(hexField(e, 'resp_ephemeral')),
  );
  final m1 = await init.writeMessage1(hexField(messages[0], 'payload'));
  await expectSameBytes('message 1', m1, hexField(messages[0], 'ciphertext'));
  await expectSameBytes(
    'payload 1',
    await resp.readMessage1(m1),
    hexField(messages[0], 'payload'),
  );
  final (m2, respT) = await resp.writeMessage2(
    hexField(messages[1], 'payload'),
  );
  await expectSameBytes('message 2', m2, hexField(messages[1], 'ciphertext'));
  final result = await init.readMessage2(m2);
  await expectSameBytes(
    'payload 2',
    result.payload,
    hexField(messages[1], 'payload'),
  );
  if (e['handshake_hash'] is String) {
    await expectSameBytes(
      'handshake hash',
      result.handshakeHash,
      hexField(e, 'handshake_hash'),
    );
  }
  for (var i = 2; i < messages.length; i++) {
    final fromInitiator = i.isEven;
    final sender = fromInitiator ? result.transport : respT;
    final receiver = fromInitiator ? respT : result.transport;
    final payload = hexField(messages[i], 'payload');
    final ct = await sender.encrypt(payload);
    await expectSameBytes(
      'transport message $i',
      ct,
      hexField(messages[i], 'ciphertext'),
    );
    await expectSameBytes(
      'transport payload $i',
      await receiver.decrypt(ct),
      payload,
    );
  }
}

void main() {
  const files = {
    'cacophony': 'cacophony.txt.gz',
    'noise-c': 'noise-c-basic.txt.gz',
  };
  files.forEach((name, file) {
    test('TM-T01 $name', () async {
      final entries = loadEntries(file);
      final chosen = [
        for (final e in entries)
          if (selected(e)) e,
      ];
      expect(
        chosen,
        isNotEmpty,
        reason: '$file yields no $noiseProtocolName entry',
      );
      for (final e in chosen) {
        if (e['fail'] == true) {
          await expectLater(replay(e), throwsA(anything));
        } else {
          await replay(e);
        }
      }
    });
  });

  test('the PSK entry of noise-c is not selected', () {
    final entries = loadEntries('noise-c-basic.txt.gz');
    final psk = entries.where(
      (e) => protocolOf(e) == 'NoisePSK_NK_25519_ChaChaPoly_SHA256',
    );
    expect(psk, isNotEmpty);
    expect(psk.where(selected), isEmpty);
  });

  test('loader rules', () {
    expect(
      protocolOf({
        'pattern': 'NK',
        'dh': '25519',
        'cipher': 'ChaChaPoly',
        'hash': 'SHA256',
      }),
      noiseProtocolName,
    );
    expect(
      selected({
        'name': noiseProtocolName,
        'init_psks': ['00'],
      }),
      isFalse,
    );
    expect(selected({'name': noiseProtocolName, 'fallback': true}), isFalse);
    expect(selected({'protocol_name': noiseProtocolName}), isTrue);
  });
}
