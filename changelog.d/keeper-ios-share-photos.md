### Changed

- The iOS share device tests no longer stand still in Photos: when a round of the walk to the
  newest photo sees none of its views for five rounds in a row, the walk scrolls the library
  towards the newest photo or opens Photos again, by turns, and a failure names these steps.
  Before, main run 40671022325 waited out the whole bound on a grid whose photos were missing
  from the accessibility tree.
