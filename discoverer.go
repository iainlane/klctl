package main

import (
	"context"
	"fmt"
	"time"

	"github.com/endocrimes/keylight-go"
)

type Discovery interface {
	Run(ctx context.Context) error
	ResultsCh() <-chan Device
}

type DiscoveryWrapper struct {
	discovery keylight.Discovery
}

func (w *DiscoveryWrapper) Run(ctx context.Context) error {
	return w.discovery.Run(ctx)
}

func (w *DiscoveryWrapper) ResultsCh() <-chan Device {
	outCh := make(chan Device)

	go func() {
		for device := range w.discovery.ResultsCh() {
			outCh <- KeylightDevice{device}
		}
		close(outCh)
	}()

	return outCh
}

type discoveryTimeoutError struct{}

func (te *discoveryTimeoutError) Error() string {
	return "timed out while discovering devices"
}

func (te *discoveryTimeoutError) ExitCode() int {
	return 1
}

func Discover(ctx context.Context, discoverer Discovery) ([]Device, error) {
	// make sure the discovery is stopped when we return from this function
	subCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(fmt.Errorf("finished discovering devices"))

	var devices []Device
	errCh := make(chan error)
	go func() {
		errCh <- discoverer.Run(subCtx)
	}()

	// keep trying until it's been a second since the last device was found or
	// we hit the global timeout, then return
	discoveryTimeout := time.NewTimer(time.Second)
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return nil, &discoveryTimeoutError{}
			}
			return nil, ctx.Err()
		case device := <-discoverer.ResultsCh():
			devices = append(devices, device)
			discoveryTimeout.Reset(time.Second)
		case <-discoveryTimeout.C:
			return devices, nil
		case err := <-errCh:
			return nil, err
		}
	}
}
