// SPDX-License-Identifier: Apache-2.0
// Jest setup provided by Grafana scaffolding
import './.config/jest-setup';

// The scaffold gives jsdom the MessageChannel of worker_threads for React 19's scheduler. Its
// port stays referenced once the scheduler sets `onmessage`, so Node's event loop never ends and
// a test file that imports @grafana/ui never exited (Jest waited after "Ran all test suites";
// several files at once left a force-exited worker). This channel delivers each message with a
// timer instead, which ends with the test run.
class TimerPort {
  constructor() {
    this.onmessage = null;
    this.other = null;
  }
  postMessage(data) {
    const target = this.other;
    setTimeout(() => target && target.onmessage && target.onmessage({ data }), 0);
  }
  start() {}
  close() {
    this.onmessage = null;
  }
  addEventListener() {}
  removeEventListener() {}
}

class TimerMessageChannel {
  constructor() {
    this.port1 = new TimerPort();
    this.port2 = new TimerPort();
    this.port1.other = this.port2;
    this.port2.other = this.port1;
  }
}

Object.assign(global, { MessageChannel: TimerMessageChannel });
