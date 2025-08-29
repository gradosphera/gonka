package types

import "math"

func (sa *SettleAmount) GetTotalCoins() int64 {
	sum := sa.RewardCoins + sa.WorkCoins
	if sum > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(sum)
}
