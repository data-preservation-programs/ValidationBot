package main

import (
	"context"
	"testing"
)

func TestRetrieveDeals(t *testing.T) {
	HandleRequest(context.TODO(), ValidationEvent{
		Cid:      "bafaxxxx",
		Provider: "f0xxx",
	})
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
}
