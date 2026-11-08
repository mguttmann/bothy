### Fixed

- Follow-up transcription (issue 574): a run that ends without a stop now also ends a running
  transcription of a follow-up instead of letting it run until `runner.transcribe_timeout`;
  stdin of the turn stays open while a follow-up is prepared, so a long transcription no
  longer turns the follow-up into a run of its own; held follow-ups are prepared while the
  turn's output is already read, and a follow-up held between the processes of a recovery is
  transcribed and submitted once; a redelivery of a follow-up being prepared keeps the order;
  the "not transcribed" note comes only once the follow-up is written, and parallel
  transcriptions share one status line (09 18.5, 18.14).
