package main

import (
	"context"

	"github.com/endocrimes/keylight-go"
)

type DeviceInfoFetcher interface {
	FetchDeviceInfo(ctx context.Context) (*keylight.DeviceInfo, error)
}

type SettingsFetcher interface {
	FetchSettings(ctx context.Context) (*keylight.DeviceSettings, error)
}

type LightGroupFetcher interface {
	FetchLightGroup(ctx context.Context) (*keylight.LightGroup, error)
}

type LightGroupUpdater interface {
	UpdateLightGroup(ctx context.Context, lg *keylight.LightGroup) (*keylight.LightGroup, error)
}

// Make sure the upstream keylight.Device implements all these interfaces
var _ DeviceInfoFetcher = &keylight.Device{}
var _ SettingsFetcher = &keylight.Device{}
var _ LightGroupFetcher = &keylight.Device{}
var _ LightGroupUpdater = &keylight.Device{}
