// SPDX-License-Identifier: Apache-2.0

/// Reading the JSON fixtures of `fixtures/` with the generated serializers.
library;

import 'dart:convert';

import 'package:api_client/api_client.dart' as api;
import 'package:built_value/serializer.dart';

/// Reads the text of one fixture file by its name, for example `StatsLiveFrame.normal.json`.
typedef FixtureReader = Future<String> Function(String name);

/// The fixtures of `bothy_viz` decoded into the generated `api_client` types.
///
/// The reader decides where the files come from: the harness reads the asset bundle
/// (`packages/bothy_viz/fixtures/<name>`), the tests read the files of the package. Decoding
/// uses the generated serializers, so a fixture that the client cannot read fails here as it
/// would on a server response (docs/spec/25-visualisierung.md 7.2, VZ-T11).
///
/// ```dart
/// final fixtures = VizFixtures((name) => rootBundle.loadString('packages/bothy_viz/fixtures/$name'));
/// final frame = await fixtures.liveFrame('normal');
/// ```
final class VizFixtures {
  /// Creates the loader around [read].
  const VizFixtures(this.read);

  /// Reads the text of a fixture file.
  final FixtureReader read;

  /// Decodes the file [name] as [serializer]; throws a [FormatException] or a
  /// `DeserializationError` of `built_value` when the file is no valid instance.
  Future<T> decode<T>(String name, Serializer<T> serializer) async {
    final json = jsonDecode(await read(name));
    final value = api.standardSerializers.deserializeWith(serializer, json);
    if (value == null) {
      throw FormatException('fixture $name decodes to null');
    }
    return value;
  }

  /// `StatsLiveFrame.<state>.json`.
  Future<api.StatsLiveFrame> liveFrame(String state) =>
      decode('StatsLiveFrame.$state.json', api.StatsLiveFrame.serializer);

  /// `StatsTimeline.<state>.json`.
  Future<api.StatsTimeline> timeline(String state) =>
      decode('StatsTimeline.$state.json', api.StatsTimeline.serializer);

  /// `StatsSloReport.<state>.json`.
  Future<api.StatsSloReport> sloReport(String state) =>
      decode('StatsSloReport.$state.json', api.StatsSloReport.serializer);

  /// `StatsBackendReport.<state>.json`.
  Future<api.StatsBackendReport> backendReport(String state) => decode(
    'StatsBackendReport.$state.json',
    api.StatsBackendReport.serializer,
  );

  /// `StatsSeries.<query>.<state>.json`, or `StatsSeries.<state>.json` without [query].
  Future<api.StatsSeries> series(String state, {String? query}) => decode(
    query == null
        ? 'StatsSeries.$state.json'
        : 'StatsSeries.$query.$state.json',
    api.StatsSeries.serializer,
  );

  /// `CostSeries.<state>.json`.
  Future<api.CostSeries> costSeries(String state) =>
      decode('CostSeries.$state.json', api.CostSeries.serializer);
}
