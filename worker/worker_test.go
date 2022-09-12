package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"validation-bot/worker/model"
)

/* The order of events emitted by a successful retrieval is:
		     * Retrieval    - code: query-asked, phase: query
			 * DataTransfer - code: Open, channel status: Requested
		     * Retrieval    - code: proposed, phase: retrieval
			 * DataTransfer - code: , channel status: Requested
			 * DataTransfer - code: DataReceived, channel status: Requested
	         * ...
			 * DataTransfer - code: DataReceived, channel status: Requested
		     * Retrieval    - code: accepted, phase: retrieval
		     * DataTransfer - code: Accept, channel status: Ongoing
		     * DataTransfer - code: PauseResponder, channel status: ResponderPaused
			 * DataTransfer - code: ResumeResponder, channel status: Ongoing
			 * DataTransfer - code: DataReceivedProgress, channel status: Ongoing
			 * Retrieval    - code: first-byte-received, phase: retrieval
			 * DataTransfer - code: DataReceived, channel status: Ongoing
			 * DataTransfer - code: DataReceivedProgress, channel status: Ongoing
	         * ...
             * DataTransfer - code: NewVoucherResult, channel status: Ongoing
             * DataTransfer - code: ResponderCompletes, channel status: ResponderCompleted
             * DataTransfer - code: DataReceivedProgress, channel status: ResponderCompleted
             * DataTransfer - code: DataReceived, channel status: ResponderCompleted
             * DataTransfer - code: FinishTransfer, channel status: Completing
             * Retrieval    - code: success, phase: retrieval
	         * DataTransfer - code: CleanupComplete, channel status: Completed
*/
/* Just retrieval event
 			 * Retrieval    - code: query-asked, phase: query
		     * Retrieval    - code: proposed, phase: retrieval
		     * Retrieval    - code: accepted, phase: retrieval
			 * Retrieval    - code: first-byte-received, phase: retrieval
             * Retrieval    - code: success, phase: retrieval
*/

func TestRetrieveDealsWithWrongType(t *testing.T) {
	result, err := HandleRequest(context.TODO(), model.ValidationInput{
		Type: "wrong_type",
	})
	if result != nil {
		t.Errorf("result should be nil")
	}
	if err == nil {
		t.Errorf("error should not be nil")
	}
}

func TestRetrievalDeals_NonQueryableMiner(t *testing.T) {
	result, err := HandleRequest(context.TODO(), model.ValidationInput{
		Type:               "retrieval_graphsync",
		MaxDurationSeconds: 600,
		MaxDownloadBytes:   100 * 1024 * 1024,
		Provider:           "f03223000",
		Cid:                "bafybeiee7fchamcdeykdnruphxjkbp5gtapao6dysa75ijdzvi3wa7wdhy",
	})
	if err != nil {
		t.Errorf("error should be nil")
	}
	if result.Success {
		t.Errorf("result should be false")
	}
	if result.ErrorCode != model.RetrievalQueryFailed {
		t.Errorf("error code should be RetrievalQueryFailed")
	}
	if result.ErrorMessage != "lotus error: failed to load miner actor: actor not found" {
		t.Errorf("error message should be 'lotus error: failed to load miner actor: actor not found'")
	}
}

func TestRetrievalDeals_InvalidCid(t *testing.T) {
	result, err := HandleRequest(context.TODO(), model.ValidationInput{
		Type:               "retrieval_graphsync",
		MaxDurationSeconds: 600,
		MaxDownloadBytes:   100 * 1024 * 1024,
		Provider:           "f03223",
		Cid:                "bafy",
	})
	if err != nil {
		t.Errorf("error should be nil")
	}
	if result.Success {
		t.Errorf("result should be false")
	}
	if result.ErrorCode != model.InternalError {
		t.Errorf("error code should be InternalError")
	}
	if !strings.Contains(result.ErrorMessage, "failed to decode cid") {
		t.Errorf("error message should be 'failed to decode cid'")
	}
}

func validateSuccessResult(result *model.ValidationResult, err error, t *testing.T) {
	if err != nil {
		t.Errorf("error should be nil")
	}
	if !result.Success {
		t.Errorf("result should be true")
	}
	if result.TimeToFirstByteMs < 1 {
		t.Errorf("time to first byte should be greater than 0")
	}
	if result.SpeedBpsAvg < 1 {
		t.Errorf("average speed should be greater than 0")
	}
	if result.SpeedBpsP50 < 1 {
		t.Errorf("speed p50 should be greater than 0")
	}
}

func TestRetrievalDeals_CompleteDownload(t *testing.T) {
	result, err := HandleRequest(context.TODO(), model.ValidationInput{
		Type:               "retrieval_graphsync",
		MaxDurationSeconds: 3600,
		MaxDownloadBytes:   32 * 1024 * 1024 * 1024,
		Provider:           "f03223",
		Cid:                "bafybeibxlbpejjvrfs7c3ytnpxyem5n4nkxbu7wecjcmrrawxuqojana3y",
	})
	fmt.Printf("result: %+v", result)
	validateSuccessResult(result, err, t)
}

func TestRetrievalDeals_Timeout(t *testing.T) {
	result, err := HandleRequest(context.TODO(), model.ValidationInput{
		Type:               "retrieval_graphsync",
		MaxDurationSeconds: 5,
		MaxDownloadBytes:   100 * 1024 * 1024,
		Provider:           "f03223",
		Cid:                "bafybeibxlbpejjvrfs7c3ytnpxyem5n4nkxbu7wecjcmrrawxuqojana3y",
	})
	fmt.Printf("result: %+v", result)
	validateSuccessResult(result, err, t)
}

func TestRetrievalDeals_ReachesMaxSize(t *testing.T) {
	result, err := HandleRequest(context.TODO(), model.ValidationInput{
		Type:               "retrieval_graphsync",
		MaxDurationSeconds: 60,
		MaxDownloadBytes:   10 * 1024 * 1024,
		Provider:           "f03223",
		Cid:                "bafybeibxlbpejjvrfs7c3ytnpxyem5n4nkxbu7wecjcmrrawxuqojana3y",
	})
	fmt.Printf("result: %+v", result)
	validateSuccessResult(result, err, t)
}
