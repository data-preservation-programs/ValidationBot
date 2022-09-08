package retrieval

import (
	"fmt"
	"github.com/application-research/filclient/rep"
	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"testing"
	"time"
	model "validation-bot/worker/model"
)

func TestOnBytesReceived(t *testing.T) {
	r := retrievalHelper{
		maxAllowedDownloadBytes: 500,
		metrics: metrics{
			retrievalEvents: make(map[rep.Code]time.Time),
			bytesReceived:   make([]timeBytesPair, 0),
		},
		done: make(chan interface{}),
	}
	go func() {
		r.onBytesReceived(100)
		r.onBytesReceived(300)
		r.onBytesReceived(600)
		r.onBytesReceived(1000)
	}()
	select {
	case x, ok := <-r.done:
		if !ok {
			t.Errorf("Expected channel to be open")
		}
		code, ok := x.(rep.Code)
		if !ok {
			t.Errorf("Expected channel to contain a rep.Code")
		}
		if code != rep.SuccessCode {
			t.Errorf("Expected channel to contain a rep.SuccessCode")
		}
	case <-time.After(5 * time.Second):
		t.Errorf("Expected channel to have an item")
	}
	if len(r.metrics.bytesReceived) != 3 {
		t.Errorf("Expected bytesReceived to have 1 item")
	}
	if r.metrics.bytesReceived[1].bytes != 300 {
		t.Errorf("Expected bytesReceived to have the correct bytes")
	}
	if r.metrics.bytesReceived[0].time == r.metrics.bytesReceived[1].time {
		t.Errorf("Expected bytesReceived to have the correct time")
	}
}

func TestOnRetrievalEvent(t *testing.T) {
	r := retrievalHelper{
		maxAllowedDownloadBytes: 0,
		metrics: metrics{
			retrievalEvents: make(map[rep.Code]time.Time),
			bytesReceived:   make([]timeBytesPair, 0),
		},
		done: make(chan interface{}),
	}
	go func() {
		r.OnRetrievalEvent(rep.NewRetrievalEventAccepted(
			rep.RetrievalPhase, cid.Cid{}, "", address.Undef))
	}()
	if _, ok := r.metrics.retrievalEvents[rep.AcceptedCode]; !ok {
		t.Errorf("Expected AcceptedCode to be present in metrics")
	}
}

func TestOnRetrievalEventWithSuccessCode(t *testing.T) {
	r := retrievalHelper{
		maxAllowedDownloadBytes: 0,
		metrics: metrics{
			retrievalEvents: make(map[rep.Code]time.Time),
			bytesReceived:   make([]timeBytesPair, 0),
		},
		done: make(chan interface{}),
	}
	go func() {
		r.OnRetrievalEvent(rep.NewRetrievalEventSuccess(
			rep.RetrievalPhase, cid.Cid{}, "", address.Undef, 100, 100))
	}()
	select {
	case x, ok := <-r.done:
		if !ok {
			t.Errorf("Expected channel to be open")
		}
		code, ok := x.(rep.Code)
		if !ok {
			t.Errorf("Expected channel to contain a rep.Code")
		}
		if code != rep.SuccessCode {
			t.Errorf("Expected channel to contain a rep.SuccessCode")
		}
	case <-time.After(5 * time.Second):
		t.Errorf("Expected channel to have an item")
	}
	if _, ok := r.metrics.retrievalEvents[rep.SuccessCode]; !ok {
		t.Errorf("Expected AcceptedCode to be present in metrics")
	}
}

func TestCalculateValidationResult_Failure(t *testing.T) {
	r := retrievalHelper{
		maxAllowedDownloadBytes: 0,
		metrics: metrics{
			retrievalEvents: make(map[rep.Code]time.Time),
			bytesReceived:   make([]timeBytesPair, 0),
		},
		done: make(chan interface{}),
	}
	r.metrics.retrievalEvents[rep.FailureCode] = time.Now()
	result := r.CalculateValidationResult()
	if result.Success {
		t.Errorf("Expected result to be false")
	}
	if result.ErrorCode != model.RetrievalFailed {
		t.Errorf("Expected result to have the correct error code")
	}
}

func TestCalculateValidationResult_NoDownload(t *testing.T) {
	r := retrievalHelper{
		maxAllowedDownloadBytes: 0,
		metrics: metrics{
			retrievalEvents: make(map[rep.Code]time.Time),
			bytesReceived:   make([]timeBytesPair, 0),
		},
		done: make(chan interface{}),
	}
	now := time.Now()
	r.metrics.retrievalEvents[rep.AcceptedCode] = now
	r.metrics.retrievalEvents[rep.FirstByteCode] = now.Add(1 * time.Second)
	r.metrics.retrievalEvents[rep.SuccessCode] = now.Add(2 * time.Second)
	result := r.CalculateValidationResult()
	if !result.Success {
		t.Errorf("Expected result to be true")
	}
	if result.TimeToFirstByteMs != 1000 {
		t.Errorf("Expected result to have the correct time to first byte")
	}
	if result.AverageSpeedBps != 0 {
		t.Errorf("Expected result to have the correct average speed")
	}
	if result.SpeedBpsP50 != 0 {
		t.Errorf("Expected result to have the correct p50 speed")
	}
}

func TestCalculateValidationResult_WithDownload(t *testing.T) {
	r := retrievalHelper{
		maxAllowedDownloadBytes: 0,
		metrics: metrics{
			retrievalEvents: make(map[rep.Code]time.Time),
			bytesReceived:   make([]timeBytesPair, 0),
		},
		done: make(chan interface{}),
	}
	now := time.Now()
	r.metrics.retrievalEvents[rep.AcceptedCode] = now
	r.metrics.retrievalEvents[rep.FirstByteCode] = now.Add(1 * time.Second)
	r.metrics.retrievalEvents[rep.SuccessCode] = now.Add(2 * time.Second)
	received := 0
	for i := 0; i < 100; i++ {
		received += 200 - i
		r.metrics.bytesReceived = append(r.metrics.bytesReceived, timeBytesPair{
			time:  now.Add(time.Duration(i) * time.Second),
			bytes: uint64(received),
		})
	}
	result := r.CalculateValidationResult()
	fmt.Printf("%+v", result)
	if !result.Success {
		t.Errorf("Expected result to be true")
	}
	if result.TimeToFirstByteMs != 1000 {
		t.Errorf("Expected result to have the correct time to first byte")
	}
	if result.AverageSpeedBps < 153 || result.AverageSpeedBps > 154 {
		t.Errorf("Expected result to have the correct average speed")
	}
	if result.SpeedBpsP1 != 101 {
		t.Errorf("Expected result to have the correct p1 speed")
	}
	if result.SpeedBpsP10 != 110 {
		t.Errorf("Expected result to have the correct p10 speed")
	}
	if result.SpeedBpsP50 != 150 {
		t.Errorf("Expected result to have the correct p50 speed")
	}
	if result.SpeedBpsP90 != 190 {
		t.Errorf("Expected result to have the correct p90 speed")
	}
}
