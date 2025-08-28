package utils

import (
	"decentralized-api/completionapi"
	"decentralized-api/logging"
	"decentralized-api/utils"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/productscience/inference/x/inference/types"
)

// UnquoteEventValue removes JSON quotes from event values
// Cosmos SDK events often have JSON-encoded values like "\"1\"" which need to be unquoted to "1"
func UnquoteEventValue(value string) (string, error) {
	var unquoted string
	err := json.Unmarshal([]byte(value), &unquoted)
	if err != nil {
		return value, nil // Return original value if unquoting fails
	}
	return unquoted, nil
}

// DecodeBase64IfPossible attempts to decode a string as base64
// Returns the decoded bytes if successful, or an error if not valid base64
func DecodeBase64IfPossible(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// DecodeHex decodes a hex string to bytes
// Returns the decoded bytes if successful, or an error if not valid hex
func DecodeHex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

func GetResponseHash(bodyBytes []byte) (string, *completionapi.Response, error) {
	var response completionapi.Response
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return "", nil, err
	}

	var content string
	for _, choice := range response.Choices {
		content += choice.Message.Content
	}
	hash := utils.GenerateSHA256Hash(content)
	return hash, &response, nil
}

// ComputeValidationWaitBlocks computes the minimum of epochLength/4 and 40 blocks
func ComputeValidationWaitBlocks(epochLength int64) int64 {
	var waitBlocks int64 = epochLength / 4
	if waitBlocks > 40 {
		waitBlocks = 40
	}
	return waitBlocks
}

// WaitNBlocks waits for a specified number of blocks using block height monitoring
func WaitNBlocks(nBlocks int64, getHeight func() int64) {
	startTime := time.Now()
	logging.Info("Starting block wait", types.Validation, "targetBlocks", nBlocks)

	// Get starting block height
	startHeight := getHeight()
	targetHeight := startHeight + nBlocks

	// Wait for target number of blocks
	for {
		currentHeight := getHeight()
		if currentHeight >= targetHeight {
			waitDuration := time.Since(startTime)
			logging.Info("Block wait completed", types.Validation,
				"startHeight", startHeight,
				"currentHeight", currentHeight,
				"targetHeight", targetHeight,
				"blocksWaited", currentHeight-startHeight,
				"timeWaitedSeconds", int(waitDuration.Seconds()))
			break
		}

		// Sleep for a short interval before checking again
		time.Sleep(5 * time.Second)
	}
}
