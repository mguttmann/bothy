### Fixed

- The executive page of "Live" shows failure rates in percent. It showed them a hundred times
  too small, so a failure rate of 14 % read "0 %", and the rate of disabled push registrations
  read as a rounded count.
- The executive page names the period of its figures under its title and asks the server for
  exactly that period: the last 24 hours in whole hours. Before, the figures covered two calendar
  days without saying so. Every count is its sum over the period; a value that is the level of
  its latest hour, such as a failure rate, says since when.
- The executive page shows the active people of the current week when its scope has one space,
  and "fewer than the minimum group" where the server withholds the number. The tile never
  appeared before.
- Costs in several currencies no longer replace the whole executive page by an error: the page
  shows the first currency with a note.
- In the run history, a "Load more runs" that fails keeps the runs already loaded and the filter,
  says why above the button, and can be pressed again.
