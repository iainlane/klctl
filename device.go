package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/endocrimes/keylight-go"
)

type Device interface {
	GetDNSAddr() string
	FetchDeviceInfo(ctx context.Context) (*keylight.DeviceInfo, error)
	FetchSettings(ctx context.Context) (*keylight.DeviceSettings, error)
	FetchLightGroup(ctx context.Context) (*keylight.LightGroup, error)
	UpdateLightGroup(ctx context.Context, lg *keylight.LightGroup) (*keylight.LightGroup, error)
}

// KeylightDevice is a wrapper around keylight.Device that implements the
// interface above. This allows us to use the upstream keylight.Device directly,
// but also to mock it out in tests, including property accessors.
type KeylightDevice struct {
	*keylight.Device
}

func (device KeylightDevice) GetDNSAddr() string {
	return device.DNSAddr
}

func DeviceString(
	device Device,
	info keylight.DeviceInfo,
	settings keylight.DeviceSettings,
	lightGroup keylight.LightGroup,
) string {
	var sb strings.Builder

	sb.WriteString("Device: ")
	sb.WriteString(device.GetDNSAddr())
	sb.WriteString("\n")
	sb.WriteString("DeviceInfo: ")
	sb.WriteString(fmt.Sprintf("%+v", info))
	sb.WriteString("\n")
	sb.WriteString("DeviceSettings: ")
	sb.WriteString(fmt.Sprintf("%+v", settings))
	sb.WriteString("\n")
	sb.WriteString("LightGroup: ")
	for _, light := range lightGroup.Lights {
		sb.WriteString(fmt.Sprintf("%+v", light))
	}

	return sb.String()
}

// Make sure the upstream keylight.Device implements this interface.
var _ Device = &KeylightDevice{}
