package types

// returns true if we've gotten data we can only get from both StartInference and FinishInference
func (i *Inference) IsCompleted() bool {
	return i.Model != "" && i.RequestedBy != "" && i.ExecutedBy != ""
}

func (i *Inference) StartProcessed() bool {
	return i.PromptHash != ""
}

func (i *Inference) FinishedProcessed() bool {
	return i.ExecutedBy != ""
}
