package types

func (egd *EpochGroupData) IsModelGroup() bool {
	return egd.ModelId != ""
}

func (egd *EpochGroupData) ValidationWeight(memberAddress string) *ValidationWeight {
	for _, member := range egd.ValidationWeights {
		if member.MemberAddress == memberAddress {
			return member
		}
	}
	return nil
}
