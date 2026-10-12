// SPDX-License-Identifier: Apache-2.0

// TM-T02 (docs/spec/24-terminal.md 14): Dart writes recordings/dart.json and replays the Go
// recording recordings/go.json as initiator and as responder. Format: brief M2-38, Design
// note 11. UPDATE_RECORDINGS=1 writes the Dart recording instead of comparing it.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:bothy_noise/testing.dart';
import 'package:cryptography/cryptography.dart';
import 'package:test/test.dart';

import 'support.dart';

const dartSession = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e11';
const dartHost = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e12';
const message2Payload = '{"v":1,"host_version":"0.0.0","profile":"shell"}';
const scenarioNames = ['open_shell', 'attach_replay', 'limits'];

typedef Scripted = (FrameSender, TerminalFrame);

List<Scripted> script(String name) {
  const dev = FrameSender.device;
  const host = FrameSender.host;
  Uint8List text(String s) => Uint8List.fromList(utf8.encode(s));
  switch (name) {
    case 'open_shell':
      return [
        (dev, const StartFrame(120, 40)),
        (dev, DataFrame(text('ls\n'))),
        (host, DataFrame(text('README.md\r\n'))),
        (dev, const ResizeFrame(100, 30)),
        (dev, DataFrame(text('exit\n'))),
        (host, const ExitFrame(0)),
      ];
    case 'attach_replay':
      return [
        (dev, const StartFrame(80, 24)),
        (host, const ReplayBeginFrame()),
        (host, DataFrame(text('earlier output\r\n'))),
        (host, const ReplayEndFrame()),
        (dev, const CloseFrame()),
      ];
    default:
      final big = Uint8List.fromList(
        List<int>.generate(maxFrameLength - 1, (i) => i % 251),
      );
      return [
        (dev, const StartFrame(65535, 65535)),
        (dev, DataFrame(big)),
        (host, const ExitFrame(-1)),
      ];
  }
}

Future<Map<String, Object?>> generateScenario(String name) async {
  final rsPriv = await sha256Of('noise-recording/dart/$name/responder-static');
  final iePriv = await sha256Of(
    'noise-recording/dart/$name/initiator-ephemeral',
  );
  final rePriv = await sha256Of(
    'noise-recording/dart/$name/responder-ephemeral',
  );
  final rs = await keyPairOf(rsPriv);
  final ie = await keyPairOf(iePriv);
  final re = await keyPairOf(rePriv);
  final m1Payload = utf8.encode('EXAMPLE session token $name');
  final m2Payload = utf8.encode(message2Payload);
  final init = await NkInitiator.start(
    hostStaticPublicKey: await publicOf(rs),
    sessionId: uuidToBytes(dartSession),
    hostId: uuidToBytes(dartHost),
    ephemeral: ie,
  );
  final resp = await NkResponder.start(
    staticKeyPair: rs,
    sessionId: uuidToBytes(dartSession),
    hostId: uuidToBytes(dartHost),
    ephemeral: re,
  );
  final m1 = await init.writeMessage1(m1Payload);
  await resp.readMessage1(m1);
  final (m2, hostT) = await resp.writeMessage2(m2Payload);
  final result = await init.readMessage2(m2);
  final transport = <Map<String, Object?>>[];
  for (final (from, frame) in script(name)) {
    final bytes = encodeFrame(frame);
    final sender = from == FrameSender.device ? result.transport : hostT;
    transport.add({
      'from': from.name,
      'frame': toHex(bytes),
      'message': toHex(await sender.encrypt(bytes)),
    });
  }
  return {
    'name': name,
    'session_id': dartSession,
    'host_id': dartHost,
    'responder_static_private': toHex(rsPriv),
    'responder_static_public': toHex(await publicOf(rs)),
    'initiator_ephemeral_private': toHex(iePriv),
    'initiator_ephemeral_public': toHex(await publicOf(ie)),
    'responder_ephemeral_private': toHex(rePriv),
    'responder_ephemeral_public': toHex(await publicOf(re)),
    'message1_payload': toHex(m1Payload),
    'message1': toHex(m1),
    'message2_payload': toHex(m2Payload),
    'message2': toHex(m2),
    'handshake_hash': toHex(result.handshakeHash),
    'transport': transport,
  };
}

Future<String> generateRecording() async {
  final scenarios = [for (final n in scenarioNames) await generateScenario(n)];
  final rec = {
    'format': 1,
    'protocol': noiseProtocolName,
    'generator': 'dart',
    'scenarios': scenarios,
  };
  return '${const JsonEncoder.withIndent('  ').convert(rec)}\n';
}

File recordingFile(String generator) =>
    File('${testdataDir().path}/recordings/$generator.json');

List<Map<String, Object?>> loadScenarios(String generator) {
  final rec = jsonDecode(
    recordingFile(generator).readAsStringSync(),
  ) as Map<String, Object?>;
  expect(rec['format'], 1);
  expect(rec['protocol'], noiseProtocolName);
  expect(rec['generator'], generator);
  final scenarios = (rec['scenarios']! as List<Object?>)
      .cast<Map<String, Object?>>();
  expect(scenarios, hasLength(scenarioNames.length));
  return scenarios;
}

Uint8List field(Map<String, Object?> sc, String key) =>
    fromHex(sc[key]! as String);

List<Map<String, Object?>> transportOf(Map<String, Object?> sc) =>
    (sc['transport']! as List<Object?>).cast<Map<String, Object?>>();

Future<SimpleKeyPair> recordedPair(Map<String, Object?> sc, String role) async {
  final kp = await keyPairOf(field(sc, '${role}_private'));
  await expectSameBytes(
    '$role public key',
    await publicOf(kp),
    field(sc, '${role}_public'),
  );
  return kp;
}

Future<void> replayAsInitiator(Map<String, Object?> sc) async {
  final init = await NkInitiator.start(
    hostStaticPublicKey: field(sc, 'responder_static_public'),
    sessionId: uuidToBytes(sc['session_id']! as String),
    hostId: uuidToBytes(sc['host_id']! as String),
    ephemeral: await recordedPair(sc, 'initiator_ephemeral'),
  );
  await expectSameBytes(
    'message 1',
    await init.writeMessage1(field(sc, 'message1_payload')),
    field(sc, 'message1'),
  );
  final result = await init.readMessage2(field(sc, 'message2'));
  await expectSameBytes(
    'message 2 payload',
    result.payload,
    field(sc, 'message2_payload'),
  );
  await expectSameBytes(
    'handshake hash',
    result.handshakeHash,
    field(sc, 'handshake_hash'),
  );
  for (final m in transportOf(sc)) {
    final frameBytes = fromHex(m['frame']! as String);
    if (m['from'] == 'device') {
      await expectSameBytes(
        'device message',
        await result.transport.encrypt(frameBytes),
        fromHex(m['message']! as String),
      );
    } else {
      final pt = await result.transport.decrypt(
        fromHex(m['message']! as String),
      );
      await expectSameBytes('host frame', pt, frameBytes);
      decodeFrame(pt, FrameSender.host);
    }
  }
}

Future<void> replayAsResponder(Map<String, Object?> sc) async {
  final resp = await NkResponder.start(
    staticKeyPair: await recordedPair(sc, 'responder_static'),
    sessionId: uuidToBytes(sc['session_id']! as String),
    hostId: uuidToBytes(sc['host_id']! as String),
    ephemeral: await recordedPair(sc, 'responder_ephemeral'),
  );
  await expectSameBytes(
    'message 1 payload',
    await resp.readMessage1(field(sc, 'message1')),
    field(sc, 'message1_payload'),
  );
  await expectSameBytes(
    'initiator ephemeral key',
    resp.initiatorEphemeralPublicKey,
    field(sc, 'initiator_ephemeral_public'),
  );
  final (m2, hostT) = await resp.writeMessage2(field(sc, 'message2_payload'));
  await expectSameBytes('message 2', m2, field(sc, 'message2'));
  for (final m in transportOf(sc)) {
    final frameBytes = fromHex(m['frame']! as String);
    if (m['from'] == 'host') {
      await expectSameBytes(
        'host message',
        await hostT.encrypt(frameBytes),
        fromHex(m['message']! as String),
      );
    } else {
      final pt = await hostT.decrypt(fromHex(m['message']! as String));
      await expectSameBytes('device frame', pt, frameBytes);
      decodeFrame(pt, FrameSender.device);
    }
  }
}

Matcher get handshakeFails => throwsA(
  isA<NoiseException>().having(
    (e) => e.error,
    'error',
    NoiseError.handshakeFailed,
  ),
);

Future<void> negativeCases(Map<String, Object?> sc) async {
  final rs = await recordedPair(sc, 'responder_static');
  final ie = await recordedPair(sc, 'initiator_ephemeral');
  final wrongStatic = await keyPairOf(
    await sha256Of('noise-recording/wrong-static'),
  );
  final sid = uuidToBytes(sc['session_id']! as String);
  final hid = uuidToBytes(sc['host_id']! as String);
  final flipped = Uint8List.fromList(sid)..[15] ^= 0x01;
  final m1 = field(sc, 'message1');
  final m2 = field(sc, 'message2');

  for (final (static, s, h) in [
    (rs, flipped, hid),
    (wrongStatic, sid, hid),
    (rs, hid, sid),
  ]) {
    final resp = await NkResponder.start(
      staticKeyPair: static,
      sessionId: s,
      hostId: h,
    );
    await expectLater(resp.readMessage1(m1), handshakeFails);
  }
  for (final (hostKey, s) in [
    (await publicOf(rs), flipped),
    (await publicOf(wrongStatic), sid),
  ]) {
    final init = await NkInitiator.start(
      hostStaticPublicKey: hostKey,
      sessionId: s,
      hostId: hid,
      ephemeral: ie,
    );
    await init.writeMessage1(field(sc, 'message1_payload'));
    await expectLater(init.readMessage2(m2), handshakeFails);
  }
  // Only the correct configuration succeeds.
  final resp = await NkResponder.start(
    staticKeyPair: rs,
    sessionId: sid,
    hostId: hid,
  );
  await resp.readMessage1(m1);
  final init = await NkInitiator.start(
    hostStaticPublicKey: await publicOf(rs),
    sessionId: sid,
    hostId: hid,
    ephemeral: ie,
  );
  await init.writeMessage1(field(sc, 'message1_payload'));
  await init.readMessage2(m2);
}

void main() {
  test('Dart recording is current', () async {
    final fresh = await generateRecording();
    final file = recordingFile('dart');
    if (Platform.environment['UPDATE_RECORDINGS'] == '1') {
      file.writeAsStringSync(fresh);
      return;
    }
    expect(
      file.existsSync(),
      isTrue,
      reason: 'run UPDATE_RECORDINGS=1 flutter test test/recordings_test.dart',
    );
    await expectSameBytes(
      'recordings/dart.json',
      file.readAsBytesSync(),
      utf8.encode(fresh),
    );
  });

  test('TM-T02 replays the Go recording as initiator', () async {
    for (final sc in loadScenarios('go')) {
      await replayAsInitiator(sc);
    }
  });

  test('TM-T02 replays the Go recording as responder', () async {
    for (final sc in loadScenarios('go')) {
      await replayAsResponder(sc);
    }
  });

  test('the Dart recording replays in both roles', () async {
    for (final sc in loadScenarios('dart')) {
      await replayAsInitiator(sc);
      await replayAsResponder(sc);
    }
  });

  test(
    'TM-T02 wrong prologue, wrong static key and swapped host ID fail',
    () async {
      for (final generator in ['go', 'dart']) {
        for (final sc in loadScenarios(generator)) {
          await negativeCases(sc);
        }
      }
    },
  );
}
