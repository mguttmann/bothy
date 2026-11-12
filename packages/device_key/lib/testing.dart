// SPDX-License-Identifier: Apache-2.0

/// Test support of `device_key`: the native side of the plugin in memory
/// behind a real `DeviceKeyClient`, so tests on the host run the channel, its
/// codec and the error mapping of the plugin without a platform.
///
/// Only `test/` and `integration_test/` folders import this library
/// (docs/spec/15-client-architecture.md 3.1, rule 10).
library;

export 'src/testing/fake_device_key_host.dart'
    show FakeDeviceKeyHost, FakeHostKey;
