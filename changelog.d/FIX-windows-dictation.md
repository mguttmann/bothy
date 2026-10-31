### Fixed

- Dictation tools and other programs that insert text into the focused field with Ctrl+V reach
  the Windows composer again. Their key presses carry no scan code, so the app saw a plain "v"
  instead of the paste shortcut; the Windows app now fills in the scan code before the keys
  reach the text field.
