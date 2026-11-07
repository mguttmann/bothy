# FIX-hermes-external-jobs A Hermes run holds the external jobs its tool calls registered

Follow-up of FIX-hermes-hidden-background (#562) and FIX-hermes-result-turn-end (#573). Seen in
live use on 2026-11-07: a Hermes agent started a job of several hours with a supervisor script of
its profile. The script starts the supervisor in a new session (`start_new_session`), returns at
once, and the supervisor (its parent is then the init process) runs a delegation to another coding
agent. The Bothy run ended after 9 seconds, and no banner, step or topology entry showed the job:
the adapter reads only Hermes' registries of `delegate_task` and `terminal` with `background`, and
such a process is in neither. Rule: 09 17.12 point 8, design note
[hermes-background-work.md](../design/hermes-background-work.md) section 6.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-HEJ-1 | 2026-11-07 | A process a tool call started outside Hermes' registries registers itself as an external job of the thread: a record with the thread ID, the PID, the process's start time, a title, an optional log path and the job's start time. The helper is `bothy_hermes_adapter.jobs` (`JobRegistry.register` and `deregister`, and `python -m bothy_hermes_adapter job register\|deregister\|list\|stop`); thread and profile home are arguments (a `terminal` child of Hermes has them in `HERMES_SESSION_THREAD_ID` and `HERMES_HOME`). | Hermes gives a platform no hook for a process it does not track; a registration by the process is the one generic source. Arguments instead of the environment keep the package rule that no module reads the environment (09 17.2). |
| D-FIX-HEJ-2 | 2026-11-07 | Records live in `<profile home>/<platform key>/jobs`: the folder mode `0700`, owned by the user that runs the gateway, no link, created by the adapter at connect and by the helper only below the adapter's existing state directory; each record a file `<job ID>.json` with mode `0600`, written to a temporary name and renamed. A record that is a link, another user's, readable by others, larger than 16 KiB or not a record is skipped and logged once; a folder that fails the checks is refused. | The records decide which process the adapter signals; only the adapter's own user may write them. |
| D-FIX-HEJ-3 | 2026-11-07 | A job is live while its PID runs with the registered start time (Linux `/proc/<pid>/stat` field 22, macOS `sysctl` `KERN_PROC_PID` `p_starttime`; a zombie is not live); any other platform cannot register. A job whose process cannot be checked counts as live. | The start time tells a reused PID from the job; no subprocess is started (package rule). Counting an unreadable job as live never hides work. |
| D-FIX-HEJ-4 | 2026-11-07 | A run's work includes the live registered jobs of its thread that started at or after the run started, as background processes of 17.12 points 1 and 2: the run stays `running`, each job is a step `background_job` with the name `external_job` and its title, it counts in `/metrics/background_jobs`, keeps `progress_events` rising, and its end (the process is gone, or its start time differs) completes the step. A record removed while its step runs (the job deregistered at its end) completes the step as well. Place: `handler.py` (`_job_tasks`, `_end_vanished_jobs`). | Mirrors the processes of Hermes' registry; the run never ends while such a job runs unseen. |
| D-FIX-HEJ-5 | 2026-11-07 | Stop and every early end of the run (17.12 point 6) stop the run's live jobs: `SIGTERM`, then `SIGKILL` after 5 seconds, each signal only after the PID's start time was read again and matched; a job that leads its process group gets the signal for the whole group (a supervisor and its worker), any other job alone; the adapter's own process and process group are never signalled. | A stop must end the whole job; checking PID and start time right before each signal keeps a reused PID or a foreign group safe. |
| D-FIX-HEJ-6 | 2026-11-07 | A live job no open run covers (its run ended at a restart of the gateway, or it was registered outside a run) is not adopted by a later run: the adapter posts it once into its thread as a message of the agent without a run (09 14), with its title, PID, start time, job ID and the stop command, and keeps a mark (`<job ID>.reported`) so a restart does not post it again; a refused post is not tried again. The adapter looks every `hermes_adapter.background_poll_interval` and removes the records of ended jobs then; a stale record of a job that ended (also during a restart) is removed without a post. | 17.12 point 7 posts what a turn without a run says, so nothing stays hidden; adopting an old job into the next run would hold every later run of the thread for hours, as decision 1 of the design note rules out for old processes; killing it would end work the person asked for, because a restart of the gateway alone ends the hold. |
| D-FIX-HEJ-7 | 2026-11-07 | The log path stays on the host: it is in the record and in `job list`, never in a step, a message or a log line. | A host path says nothing to the people in the thread and is no business of the server. |

Not changed: rule 7 of 17.12 (no tool without a run) stays as it is; the registration is a tool
call's own step, so it happens inside the run. Limits: a job that does not register stays invisible
(the helper is the contract; the adapter cannot find an untracked process by itself); a job's
workers outside its process group survive a stop; the end of a job posted without a run is not
posted.

Deploy note: nothing to configure. A supervisor script of a profile calls the helper after it
started its job and removes the record when the job ends; the adapter creates the folder at its
next connect.
