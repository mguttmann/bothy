// SPDX-License-Identifier: Apache-2.0

/// The terminal seams outside a browser (the test VM): no socket opens, the pins stay in the
/// memory of the process and no link opens.
library;

import 'ports.dart';

/// The inert terminal ports of a non-web platform.
ConsoleTerminalPorts terminalPorts() => ConsoleTerminalPorts(
  connect: (_) =>
      Future.error(UnsupportedError('no terminal stream outside the browser')),
  pins: MemoryPinDatabase(),
  openLink: (_) {},
);
