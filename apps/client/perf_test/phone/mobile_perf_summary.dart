// SPDX-License-Identifier: Apache-2.0

// The evaluation of the mobile performance level (brief M5-11 Design note 5; docs/spec/
// 15-client-architecture.md 19 and 22): the phone the values come from, the fixed list of
// reference devices of 15 19 and the result of a run. The targets and their comparison are those
// of perf_summary.dart ([mobileTargets]); a run is evidence only on a reference device, so only
// a reference device that meets every target passes, a reference device that misses one fails,
// and every other device reports its values without a pass. Pure Dart: the unit test
// test/perf/mobile_perf_summary_test.dart runs it on recorded samples.

import '../perf_summary.dart';

/// The phone a run measured on, as the platform reports it (mobile_device.dart).
final class MobileDevice {
  /// A device of [platform] (`ios`, `android`, or the system of another host) by
  /// [manufacturer] with the model identifier [model] running [os].
  const MobileDevice({
    required this.platform,
    required this.manufacturer,
    required this.model,
    required this.os,
  });

  /// `ios` or `android`; another value names a host that is no phone.
  final String platform;

  /// The maker: `Apple` on iOS, `ro.product.manufacturer` on Android.
  final String manufacturer;

  /// The model identifier: the hardware identifier of iOS (`utsname.machine`, for example
  /// `iPhone17,3`), `ro.product.model` of Android (for example `Pixel 6a`).
  final String model;

  /// The system and its version, for example `Android 16 (API 36)`.
  final String os;

  /// The device in one line, for logs and the report.
  String get label => '$manufacturer $model, $os';

  /// The JSON of the device in the summary.
  Map<String, Object?> toJson() => {
    'platform': platform,
    'manufacturer': manufacturer,
    'model': model,
    'os': os,
  };
}

/// One reference device of 15 19: a model identifier whose values count as evidence.
final class ReferenceDevice {
  /// The model [model] of [manufacturer] on [platform], called [name], with the [reason] it is
  /// a reference device.
  const ReferenceDevice({
    required this.platform,
    required this.manufacturer,
    required this.model,
    required this.name,
    required this.reason,
  });

  /// `ios` or `android`.
  final String platform;

  /// The maker as the platform reports it (compared without case).
  final String manufacturer;

  /// The exact model identifier (compared as it is).
  final String model;

  /// The name in the shop, for people.
  final String name;

  /// Why this model stands for the device class of 15 19.
  final String reason;

  /// Whether [device] is this model: same platform, the same maker without regard to case and
  /// exactly this model identifier.
  bool matches(MobileDevice device) =>
      device.platform == platform &&
      device.manufacturer.toLowerCase() == manufacturer.toLowerCase() &&
      device.model == model;

  /// The JSON of the entry in the summary.
  Map<String, Object?> toJson() => {
    'model': model,
    'name': name,
    'reason': reason,
  };
}

/// The reference devices of 15 19 for the phones: "an iPhone two generations old" and "an
/// Android mid-range device with 6 GB RAM". A fixed list of model identifiers, so a run can
/// never become evidence through a label or a switch; a model that is not listed reports its
/// values as not proven.
const List<ReferenceDevice> mobileReferenceDevices = [
  ReferenceDevice(
    platform: 'ios',
    manufacturer: 'Apple',
    model: 'iPhone17,3',
    name: 'iPhone 16',
    reason:
        'The standard model of the iPhone 16 generation, two generations before the '
        'iPhone 18 generation that was current when 15 19 set the reference (September '
        '2026); the Pro models of that generation have a faster chip and would flatter the '
        'values.',
  ),
  ReferenceDevice(
    platform: 'ios',
    manufacturer: 'Apple',
    model: 'iPhone17,4',
    name: 'iPhone 16 Plus',
    reason:
        'The same chip and memory as the iPhone 16 with a larger display, so the same '
        'device class with more pixels to draw per frame.',
  ),
  ReferenceDevice(
    platform: 'android',
    manufacturer: 'Google',
    model: 'Pixel 6a',
    name: 'Google Pixel 6a',
    reason:
        'A mid-range phone that exists only with 6 GB RAM, so its model identifier alone '
        'fixes the memory of 15 19; the mid-range models of other makers share one model '
        'identifier between several memory sizes.',
  ),
];

/// The entry of [references] that [device] matches, or null when it is no reference device.
ReferenceDevice? referenceDeviceOf(
  MobileDevice device, [
  List<ReferenceDevice> references = mobileReferenceDevices,
]) {
  for (final r in references) {
    if (r.matches(device)) return r;
  }
  return null;
}

/// The result of a run of the mobile performance level.
enum MobilePerfResult {
  /// A reference device met every target: evidence for 15 19.
  pass('pass'),

  /// A reference device missed a target, or a target was not measured.
  fail('fail'),

  /// The device is no reference device: the values are reported, nothing is proven.
  notProven('not_proven');

  const MobilePerfResult(this.json);

  /// The value of `result` in the summary.
  final String json;
}

/// The summary of one run on one phone.
final class MobilePerfSummary {
  MobilePerfSummary._(this.device, this.reference, this.perf);

  /// Evaluates [samples] (by target name, in the unit of the target) of a run on [device]
  /// against [targets] like [PerfSummary.evaluate] (p95 by nearest rank over at least the
  /// samples of the target, a target without samples fails) and decides the result with the
  /// reference list [references]. [details] go to the summary as they are.
  factory MobilePerfSummary.evaluate({
    required List<PerfTarget> targets,
    required MobileDevice device,
    required Map<String, List<double>> samples,
    Map<String, Object?> details = const {},
    List<ReferenceDevice> references = mobileReferenceDevices,
  }) {
    final reference = referenceDeviceOf(device, references);
    return MobilePerfSummary._(
      device,
      reference,
      PerfSummary.evaluate(
        targets: targets,
        device: device.label,
        reference: reference != null,
        samples: samples,
        details: details,
      ),
    );
  }

  /// The phone the values come from.
  final MobileDevice device;

  /// The reference entry [device] matches, or null.
  final ReferenceDevice? reference;

  /// The values and the comparison with the targets.
  final PerfSummary perf;

  /// Pass only on a reference device with every target met; not proven on any other device.
  MobilePerfResult get result => reference == null
      ? MobilePerfResult.notProven
      : perf.pass
      ? MobilePerfResult.pass
      : MobilePerfResult.fail;

  /// The JSON of build/perf/mobile-summary.json: `pass` is true only for the result `pass`;
  /// `targets_met` tells whether the values met the targets, on any device.
  Map<String, Object?> toJson() {
    final values = perf.toJson();
    return {
      'device': device.toJson(),
      'reference_device': reference != null,
      'reference': ?reference?.toJson(),
      'result': result.json,
      'pass': result == MobilePerfResult.pass,
      'targets_met': perf.pass,
      'results': values['results'],
      'details': ?values['details'],
    };
  }
}
