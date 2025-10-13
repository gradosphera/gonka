package types

import (
	"testing"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/productscience/inference/testutil/sample"
	"github.com/stretchr/testify/require"
)

func TestMsgRemoveUserFromTrainingAllowList_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgRemoveUserFromTrainingAllowList
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgRemoveUserFromTrainingAllowList{
				Authority: sample.AccAddress(),
				Address:   "invalid address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: MsgRemoveUserFromTrainingAllowList{
				Authority: sample.AccAddress(),
				Address:   sample.AccAddress(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}
