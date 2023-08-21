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

	discoveryTimeout := time.NewTimer(time.Second)
	for {
		select {
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
