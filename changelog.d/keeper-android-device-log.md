### Changed

- `make apps-platform` prints the end of the device log (`adb logcat`, cleared before each
  attempt) after a failed Android attempt. The test app's VM service disappeared right after the
  install in main runs also after the adb restart of PL/51, and the job log showed nothing of the
  device, so a crash or a killed app process could not be told apart from a lost connection.
