package main

import (
	"context"
	"time"

	"github.com/endocrimes/keylight-go"
)

type Discoverer interface {
	Discover(ctx context.Context) ([]*keylight.Device, error)
}

type RealDiscoverer struct{}

type discoveryTimeoutError struct{}

func (te *discoveryTimeoutError) Error() string {
	return "timed out while discovering devices"
}

func (te *discoveryTimeoutError) ExitCode() int {
	return 1
}

func (rd *RealDiscoverer) Discover(ctx context.Context) ([]*keylight.Device, error) {
	discovery, err := keylight.NewDiscovery()
	if err != nil {
		return nil, err
	}

	var devices []*keylight.Device
	errCh := make(chan error)
	go func() {
		errCh <- discovery.Run(ctx)
	}()

	// keep trying until it's been a second since the last device was found,
	// then return
	discoveryTimeout := time.NewTimer(time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil, &discoveryTimeoutError{}
		case device := <-discovery.ResultsCh():
			devices = append(devices, device)
			discoveryTimeout.Reset(time.Second)
		case <-discoveryTimeout.C:
			return devices, nil
		case err := <-errCh:
			return nil, err
		}
	}
}
