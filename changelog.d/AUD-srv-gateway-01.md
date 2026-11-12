### Fixed

- Desktop notifications of the Windows and macOS apps name the channel, the thread and the
  author and show the text again in an installation with separate `api` and `worker`
  containers, the layout of the Compose bundle. Before, such a notification arrived without any
  of them, whatever the preview setting of the person, and the apps showed a placeholder.
- The run cards of the control room get the live updates of their runs. The server ignored the
  list of card threads that an app sends, so a card kept the state it was loaded with.

### Security

- Wrong credentials on the WebSocket connection of the apps, agents and SDKs now count against
  the login rate limit of the client address, like wrong credentials on every other request. An
  address that keeps sending them is refused for a while, with the wait in the close reason.
