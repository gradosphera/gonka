package types

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	BaseCoin   = "ngonka"
	NanoCoin   = "ngonka"
	NativeCoin = "gonka"
	MilliCoin  = "mgonka"
	MicroCoin  = "ugonka"
)

// NOTE: In ALL cases, if we represent coins as an int, they should be in BaseCoin units
func GetCoins(coins int64) (sdk.Coins, error) {
	coin, err := GetCoin(coins)
	return sdk.NewCoins(coin), err
}

// Negative coins will cause a panic!
func GetCoin(coin int64) (sdk.Coin, error) {
	if coin < 0 {
		return sdk.Coin{}, ErrNegativeCoinBalance
	}
	return sdk.NewInt64Coin(BaseCoin, coin), nil
}
