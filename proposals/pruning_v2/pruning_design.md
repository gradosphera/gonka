# Pruning V2
### Problem with V1
The pruning v1 design was very basic and neither efficient or scalable. Specifically:
1. It would load ALL inferences, PoCs or PoCValidations into memory, iterate and then delete. The memory footprint could be very high.
2. It would delete ALL applicable inferences, PoCs or PoCValidations in one go. This could lead to long running transactions and table locks.
3. For Inferences, it had to iterate over ALL inferences, not just inferences for the specific epochs that are being pruned.

### Solution
1. For Pocs and PocValidations, we can:
  - Use Walk to iterate over the tables, with a limit on the number of rows to delete. This will keep memory very low (walking will not save all rows), and limit the performance hit.
  - Call Prune every EndBlock, deleting rows each time until all rows are done. Some work will need to be done to make sure this is not doing more work than it needs to each time.

2. For Inferences, we can:
 - Create a new KeySet, indexed by epochIndex/inferenceId.
 - Add a row to the new KeySet when an inference is created.
 - Now we can Walk over just the inferences for a specific epoch, and delete them in batches similarly to PoCs and PoCValidations.
 - We can delete both the Inference and the KeySet at the same time, so we can take an "delete x rows each block approach" here too.
 - Transitioning from the old scanning to the new KeySet will need to be managed with a cutoff, but will not require a migration.

3. Handling the transition:
 - New single value: "PruningTransitionEpoch"
 - For Prune (called every block): If PruningTransitionEpoch == 0, return right away
 - At first PruneInferences (old location), set it to previous epoch (the epoch with only SOME)
 - After that, Prune will: Determine oldest prune (loopback limit too), delete PruneSet entries till max.
 - Old PruneInferences will function similarly