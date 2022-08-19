package pbench

import (
	"context"
	"errors"
	"reflect"
)

type flow struct {
	c    <-chan Job
	prio int
}

var (
	// ErrInvalidPriorityValue error is returned by Input method when
	// priority value is less than or equal to 0.
	ErrInvalidPriorityValue = errors.New("ErrInvalidPriorityValue")
	ErrUnassignedPriority   = errors.New("ErrUnassignedPriority")
)

// DRR is a Deficit Round Robin scheduler, as detailed in
// https://en.wikipedia.org/wiki/Deficit_round_robin.
type DRR struct {
	flows         []flow
	outChan       chan Job
	flowsToDelete []int
	prioToFlow    map[int]chan Job
}

func NewDRR(outChan chan Job) *DRR {
	return &DRR{
		outChan:    outChan,
		prioToFlow: make(map[int]chan Job),
	}
}

func (d *DRR) Schedule(j Job) error {
	var prio int
	switch j.Request.Type {
	case SlowRequest:
		prio = 2
	case FastRequest:
		prio = 3
	default:
		return ErrInvalidPriorityValue
	}

	ch, found := d.prioToFlow[prio]
	if !found {
		return ErrUnassignedPriority
	}
	ch <- j
	return nil
}

func (d *DRR) Input(prio int) error {
	if prio <= 0 {
		return ErrInvalidPriorityValue
	}
	in := make(chan Job)
	d.flows = append(d.flows, flow{c: in, prio: prio})
	d.prioToFlow[prio] = in
	return nil
}

// Schedule actually spawns the DRR goroutine. Once Schedule is called,
// goroutine starts forwarding from input channels previously registered
// through Input method to output channel.
//
// Schedule returns ContextIsNil error if ctx is nil.
//
// DRR goroutine exits when context.Context expires or when all the input
// channels are closed. DRR goroutine closes the output channel upon termination.
func (d *DRR) Start(ctx context.Context) {
	defer func() {
		for p := range d.prioToFlow {
			close(d.prioToFlow[p])
		}
	}()
	for {
		// Wait for at least one channel to be ready
		readyIndex, value, ok := getReadyChannel(
			ctx,
			d.flows)
		if readyIndex < 0 {
			// Context expired, exit
			return
		}
	flowLoop:
		for index, flow := range d.flows {
			dc := flow.prio
			if readyIndex == index {
				if !ok {
					// Chan got closed, remove it from internal slice
					d.prepareToUnregister(index)
					continue flowLoop
				} else {
					// This chan triggered the reflect.Select statement
					// transmit its value and decrement its deficit counter
					d.outChan <- value
					dc = flow.prio - 1
				}
			}
			// Trasmit from channel until it has nothing else to send
			// or its DC reaches 0
			for i := 0; i < dc; i++ {
				//First, check if context expired
				select {
				case <-ctx.Done():
					// Context expired, exit
					return
				default:
				}
				//Then, read from input chan
				select {
				case val, ok := <-flow.c:
					if !ok {
						// Chan got closed, remove it from internal slice
						d.prepareToUnregister(index)
						continue flowLoop
					} else {
						d.outChan <- val
					}
				default:
					continue flowLoop
				}
			}
		}
		// All channel closed in this execution can now be actually removed
		last := d.unregisterFlows()
		if last {
			return
		}
	}
}

func (d *DRR) prepareToUnregister(index int) {
	d.flowsToDelete = append(d.flowsToDelete, index)
}

func (d *DRR) unregisterFlows() bool {
	oldFlows := d.flows
	d.flows = make([]flow, 0, len(oldFlows)-len(d.flowsToDelete))
oldFlowsLoop:
	for i, flow := range oldFlows {
		for _, index := range d.flowsToDelete {
			if index == i {
				continue oldFlowsLoop
			}
		}
		d.flows = append(d.flows, flow)
	}
	d.flowsToDelete = []int{}
	return len(d.flows) == 0
}

func getReadyChannel(ctx context.Context, flows []flow) (int, Job, bool) {
	cases := make([]reflect.SelectCase, 0, len(flows)+1)
	//First case is the termiantion channel for context cancellation
	c := reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}
	cases = append(cases, c)
	//Create list of SelectCase
	for _, f := range flows {
		c := reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(f.c),
		}
		cases = append(cases, c)
	}
	//Call Select on all channels
	index, value, ok := reflect.Select(cases)
	//Termination channel
	if index == 0 {
		return -1, Job{}, false
	}
	//Rescaling index (-1) because of additional termination channel
	return index - 1, value.Interface().(Job), ok
}
