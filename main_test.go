package main

import (
	"context"
	"errors"
	"testing"

	"github.com/endocrimes/keylight-go"
	"github.com/stretchr/testify/require"
)

type FakeDevice struct {
	DNSAddr                  string
	DeviceInfo               *keylight.DeviceInfo
	DeviceSet                *keylight.DeviceSettings
	LightGrp                 *keylight.LightGroup
	FetchDeviceInfoError     error
	FetchDeviceSettingsError error
	FetchLightGroupError     error
	UpdateLightGroupError    error
}

func (f *FakeDevice) GetDNSAddr() string {
	return f.DNSAddr
}

func (f *FakeDevice) FetchDeviceInfo(ctx context.Context) (*keylight.DeviceInfo, error) {
	return f.DeviceInfo, f.FetchDeviceInfoError
}

func (f *FakeDevice) FetchSettings(ctx context.Context) (*keylight.DeviceSettings, error) {
	return f.DeviceSet, f.FetchDeviceSettingsError
}

func (f *FakeDevice) FetchLightGroup(ctx context.Context) (*keylight.LightGroup, error) {
	return f.LightGrp, f.FetchLightGroupError
}

func (f *FakeDevice) UpdateLightGroup(ctx context.Context, lg *keylight.LightGroup) (*keylight.LightGroup, error) {
	return f.LightGrp, f.UpdateLightGroupError
}

// FakeDiscoverer implements keylight.Discovery
type FakeDiscoverer struct {
	Devices []Device
	Error   error

	resultsCh chan Device
}

func (fd *FakeDiscoverer) Run(ctx context.Context) error {
	if fd.Error != nil {
		return fd.Error
	}

	if fd.resultsCh == nil {
		fd.resultsCh = make(chan Device, len(fd.Devices))
	}

	for _, device := range fd.Devices {
		fd.resultsCh <- device
	}

	<-ctx.Done()
	return nil
}

func (fd *FakeDiscoverer) ResultsCh() <-chan Device {
	if fd.resultsCh == nil {
		fd.resultsCh = make(chan Device, len(fd.Devices))
	}

	return fd.resultsCh
}

func TestSetupDevices(t *testing.T) {
	ctx := context.Background()

	discoverer := &FakeDiscoverer{
		Devices: []Device{
			&FakeDevice{
				DNSAddr: "1.2.3.4",
			},
		},
	}

	// Use provided light addresses
	lightAddrs := []string{"192.168.1.1:9123"}
	devices, err := setupDevices(ctx, lightAddrs, discoverer)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, devices[0].GetDNSAddr(), "192.168.1.1")

	ctx = context.Background()

	// Discover lights when none provided
	devices, err = setupDevices(ctx, []string{}, discoverer)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, devices[0].GetDNSAddr(), "1.2.3.4")

	// No lights
	devices, err = setupDevices(ctx, []string{}, &FakeDiscoverer{})
	require.NoError(t, err)
	require.Len(t, devices, 0)

	// Timed out context
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	devices, err = setupDevices(ctx, []string{}, discoverer)
	require.ErrorIs(t, err, &discoveryTimeoutError{})
	require.Len(t, devices, 0)
	cancel()

	// Cancelled context for some other reason
	ctx, cancel = context.WithCancel(context.Background())
	cancel()

	devices, err = setupDevices(ctx, []string{}, discoverer)
	require.Equal(t, err, context.Canceled)
	require.Len(t, devices, 0)

	// Error from discovery
	ctx = context.Background()
	discoveryError := errors.New("discovery error")
	discoverer = &FakeDiscoverer{
		Error: discoveryError,
	}
	devices, err = setupDevices(ctx, []string{}, discoverer)
	require.Equal(t, err, discoveryError)
	require.Len(t, devices, 0)
}

func TestFetchLightGroups(t *testing.T) {
	ctx := context.Background()

	device := &FakeDevice{
		DNSAddr: "192.168.1.1",
		LightGrp: &keylight.LightGroup{Lights: []*keylight.Light{
			{On: 1, Brightness: 50, Temperature: 3000},
		}},
	}
	lights, err := fetchLightGroups(ctx, []Device{device})
	require.NoError(t, err)
	require.NotNil(t, lights[device])
	require.Len(t, lights[device].Lights, 1)
}

func TestGetDeviceStatus(t *testing.T) {
	for _, test := range []struct {
		name          string
		device        *FakeDevice
		expectedError bool
	}{
		{
			name: "fetch device info ok",
			device: &FakeDevice{
				DNSAddr:    "192.168.1.2",
				DeviceInfo: &keylight.DeviceInfo{ProductName: "Key Light"},
				DeviceSet: &keylight.DeviceSettings{
					PowerOnBrightness: 100,
				},
				LightGrp: &keylight.LightGroup{
					Lights: []*keylight.Light{
						{
							On: 1,
						},
					},
				},
			},
		},
		{
			name: "fetch device info error",
			device: &FakeDevice{
				DNSAddr:              "192.168.1.2",
				FetchDeviceInfoError: errors.New("fetch error"),
			},
			expectedError: true,
		},
		{
			name: "fetch device info error",
			device: &FakeDevice{
				DNSAddr:                  "192.168.1.2",
				FetchDeviceSettingsError: errors.New("fetch error"),
			},
			expectedError: true,
		},
		{
			name: "fetch device info error",
			device: &FakeDevice{
				DNSAddr:              "192.168.1.2",
				FetchLightGroupError: errors.New("fetch error"),
			},
			expectedError: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()

			info, err := getDeviceStatus(ctx, []Device{test.device})
			if test.expectedError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "fetch error")
				require.Equal(t, "", info)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, "", info)
			}
		})
	}
}

func TestSetLightState(t *testing.T) {
	ctx := context.Background()

	device := &FakeDevice{
		DNSAddr: "192.168.1.1",
		LightGrp: &keylight.LightGroup{Lights: []*keylight.Light{
			{On: 1, Brightness: 50, Temperature: 3000},
		}},
	}
	err := setLightState(ctx, []Device{device}, LightToggle)
	require.NoError(t, err)

	err = setLightState(ctx, []Device{device}, LightOff)
	require.NoError(t, err)

	err = setLightState(ctx, []Device{device}, LightOn)
	require.NoError(t, err)
}
