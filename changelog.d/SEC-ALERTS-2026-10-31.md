### Security

- The CI job `gates (security and supply chain)` checks every tracked `gradle-wrapper.jar`
  against the checksums Gradle publishes (`gradle/actions/wrapper-validation`) before any step
  can run it.
- `SECURITY.md` links the GitHub instructions for private vulnerability reporting.
- The terminal suite of the console end-to-end test refuses TLS below 1.2 for its WebSocket
  upgrade request, whatever the defaults of the Python in the browser image are.
