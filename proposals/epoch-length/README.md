# Adjust Params According to real block length

The proposal changes parameters:
- `epoch_params.epoch_length` of `inference` module from `17280` to `15391` 
- `restriction_end_block` of `restrictions` module from `1555200` to `1385263`
- `blocks_per_year` of `mint` module from `6307200` to `5618012`

## Motivation

The initial values for epoch_params.epoch_length and restriction_end_block were calculated based on an estimated block time of 5 seconds per block. 
In practice, the average block time is closer to 5.61 seconds. 
This has led to epochs lasting approximately 27 hours instead of the intended 24 hours, and has similarly extended the transfer restriction period.

## New values estimations

At `/chain-rpc/status` we can get (for genesis nodes):  
```
      "latest_block_height": "223562",
      "latest_block_time": "2025-09-05T21:17:32.061238117Z",
      ...
      "earliest_block_height": "1",
      "earliest_block_time": "2025-08-22T08:42:00.713839Z",
```


Then:
- `sec_per_day = 24*60*60 = 86400` 
- `sec_per_block = (latest_block_time - earliest_block_time) / (latest_block_height - earliest_block_height) = 5.613373295874505`
- `epoch_params.epoch_length = 1 * sec_per_day // sec_per_block = 15391`
- `restriction_end_block = 90 * sec_per_day // sec_per_block = 1385263`
- `blocks_per_year = 365 * sec_per_day // sec_per_block = 5618012`

