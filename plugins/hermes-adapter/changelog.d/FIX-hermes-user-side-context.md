### Fixed

- The format rules, the channel instructions (the operator's channel prompt and skills and the
  profile's `channel_prompts`) and the run context now reach the model on the user side of the
  turn, through Hermes' `pre_llm_call` hook, instead of the ephemeral system prompt, which a host
  may shorten (on the Anthropic OAuth route a budget cut all of it). The context is shortened to
  fit Hermes' `hooks.output_spill.max_chars`, keeping the newest context messages and naming what it
  left out. Follow-ups already delivered to the session stay in the context, marked as such.
