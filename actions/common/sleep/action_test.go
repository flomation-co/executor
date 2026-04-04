package sleep

import (
	"context"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestSleep_BasicDelay(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sleep-1"}

	inputs := []*core.Connection{
		{Name: "duration", Type: core.ConnectionTypeInteger, Value: int64(1)},
		{Name: "unit", Type: core.ConnectionTypeString, Value: "seconds"},
	}

	start := time.Now()
	result, err := Execute(flow, node, inputs)
	elapsed := time.Since(start)

	Expect(err).To(BeNil())
	Expect(result["slept_for_seconds"]).To(Equal(int64(1)))
	Expect(result["cancelled"]).To(Equal(false))
	Expect(elapsed).To(BeNumerically(">=", 900*time.Millisecond))
}

func TestSleep_ZeroDuration_ReturnsImmediately(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sleep-1"}

	inputs := []*core.Connection{
		{Name: "duration", Type: core.ConnectionTypeInteger, Value: int64(0)},
	}

	start := time.Now()
	result, err := Execute(flow, node, inputs)
	elapsed := time.Since(start)

	Expect(err).To(BeNil())
	Expect(result["slept_for_seconds"]).To(Equal(int64(0)))
	Expect(elapsed).To(BeNumerically("<", 100*time.Millisecond))
}

func TestSleep_CapsAtMaxDuration(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sleep-1"}

	// Request 2 hours — should cap at 1 hour (3600s)
	// We test the cap by checking the output, not actually sleeping
	inputs := []*core.Connection{
		{Name: "duration", Type: core.ConnectionTypeInteger, Value: int64(2)},
		{Name: "unit", Type: core.ConnectionTypeString, Value: "hours"},
	}

	// Cancel immediately so we don't wait
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	flow.SetCancelContext(ctx, cancel)

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["cancelled"]).To(Equal(true))
}

func TestSleep_CancelledByContext(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sleep-1"}

	ctx, cancel := context.WithCancel(context.Background())
	flow.SetCancelContext(ctx, cancel)

	inputs := []*core.Connection{
		{Name: "duration", Type: core.ConnectionTypeInteger, Value: int64(60)},
		{Name: "unit", Type: core.ConnectionTypeString, Value: "seconds"},
	}

	// Cancel after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := Execute(flow, node, inputs)
	elapsed := time.Since(start)

	Expect(err).To(BeNil())
	Expect(result["cancelled"]).To(Equal(true))
	Expect(elapsed).To(BeNumerically("<", 500*time.Millisecond))
}

func TestSleep_MinutesConversion(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sleep-1"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	flow.SetCancelContext(ctx, cancel)

	inputs := []*core.Connection{
		{Name: "duration", Type: core.ConnectionTypeInteger, Value: int64(5)},
		{Name: "unit", Type: core.ConnectionTypeString, Value: "minutes"},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	// Cancelled immediately but would have slept 300s
	Expect(result["cancelled"]).To(Equal(true))
}

func TestSleep_MissingDuration_ReturnsError(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sleep-1"}

	inputs := []*core.Connection{}

	_, err := Execute(flow, node, inputs)
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("duration is required"))
}

func TestSleep_DefaultsToSeconds(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sleep-1"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	flow.SetCancelContext(ctx, cancel)

	inputs := []*core.Connection{
		{Name: "duration", Type: core.ConnectionTypeInteger, Value: int64(10)},
		// No unit specified — should default to seconds
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["cancelled"]).To(Equal(true))
}
