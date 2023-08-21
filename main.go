package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/endocrimes/keylight-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

type LightState int
type LightControlField int

const (
	LightOff LightState = iota
	LightOn
	LightToggle
)

func (ls LightState) String() string {
	switch ls {
	case LightOff:
		return "off"
	case LightOn:
		return "on"
	case LightToggle:
		return "toggle"
	}

	return ""
}

const (
	ControlBrightness LightControlField = iota
	ControlTemperature
)

const defaultPort = "9123"

var (
	logLevel string
	timeout  int
)

func setupDevices(ctx context.Context, lightAddrs []string, discoverer Discovery) ([]Device, error) {
	var devices []Device

	for _, lightAddr := range lightAddrs {
		host, port, err := net.SplitHostPort(lightAddr)
		if err != nil {
			host = lightAddr
			port = defaultPort
		}

		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("port must be a number between 1 and 65535 (got %s)", port)
		}

		device := KeylightDevice{
			&keylight.Device{
				DNSAddr: host,
				Port:    p,
			},
		}
		devices = append(devices, device)
	}

	if len(devices) == 0 {
		logrus.Debug("No lights provided, running discovery")
		return Discover(ctx, discoverer)
	}
	return devices, nil
}

func main() {
	lightList := []Device{}

	lightAddrs := cli.NewStringSlice()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var cancel context.CancelFunc

	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:        "light",
				Usage:       "Light to control (host:port)",
				Destination: lightAddrs,
			},
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Level of logging",
				Value:       "info",
				Destination: &logLevel,
			},
			&cli.IntFlag{
				Name:        "timeout",
				Usage:       "Timeout in seconds for operations",
				Value:       10,
				Destination: &timeout,
			},
		},

		Before: func(c *cli.Context) error {
			level, err := logrus.ParseLevel(logLevel)
			if err != nil {
				return err
			}

			logrus.SetLevel(level)

			if c.NArg() == 0 {
				return nil
			}

			ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)

			discovery, err := keylight.NewDiscovery()
			if err != nil {
				return fmt.Errorf("failed to create discovery client: %w", err)
			}

			lightList, err = setupDevices(ctx, lightAddrs.Value(), &DiscoveryWrapper{discovery})
			if err != nil {
				cancel()
				return err
			}

			return nil
		},

		After: func(c *cli.Context) error {
			if cancel != nil {
				cancel()
			}
			return nil
		},

		Commands: []*cli.Command{
			{
				Name:   "toggle",
				Usage:  "Toggle lights on and off",
				Action: func(c *cli.Context) error { return setLightState(ctx, lightList, LightToggle) },
			},
			{
				Name:   "on",
				Usage:  "Turn lights on",
				Action: func(c *cli.Context) error { return setLightState(ctx, lightList, LightOn) },
			},
			{
				Name:   "off",
				Usage:  "Turn lights off",
				Action: func(c *cli.Context) error { return setLightState(ctx, lightList, LightOff) },
			},
			{
				Name:        "brightness",
				Usage:       "Control light brightness",
				Subcommands: makeLightControlSubcommands(ctx, lightList, ControlBrightness),
			},
			{
				Name:        "temperature",
				Usage:       "Control light temperature",
				Subcommands: makeLightControlSubcommands(ctx, lightList, ControlTemperature),
			},
			{
				Name:  "status",
				Usage: "Get device information",
				Action: func(c *cli.Context) error {
					status, err := getDeviceStatus(ctx, lightList)
					if err != nil {
						return err
					}

					fmt.Println(status)

					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		if err == context.Canceled {
			logrus.Info("Interrupted")
			return
		}
		logrus.Fatal(err)
	}
}

func fetchLightGroups(ctx context.Context, lights []Device) (map[Device]*keylight.LightGroup, error) {
	lgs := make(map[Device]*keylight.LightGroup)

	for _, device := range lights {
		logrus.WithField("address", device.GetDNSAddr()).Debug("Fetching light group")
		lg, err := device.FetchLightGroup(ctx)
		if err != nil {
			return nil, err
		}

		lgs[device] = lg
	}

	return lgs, nil
}

func setLightState(ctx context.Context, lightList []Device, state LightState) error {
	lgs, err := fetchLightGroups(ctx, lightList)
	if err != nil {
		return err
	}

	for device, lightGroup := range lgs {
		for _, light := range lightGroup.Lights {
			switch state {
			case LightToggle:
				light.On = 1 - light.On
			case LightOff:
				light.On = 0
			case LightOn:
				light.On = 1
			}

			logrus.WithFields(logrus.Fields{
				"address": device.GetDNSAddr(),
				"state":   LightState(light.On),
			}).Debug("Updating light")
		}

		_, err = device.UpdateLightGroup(ctx, lightGroup)
		if err != nil {
			return err
		}
	}

	return nil
}

func makeLightControlSubcommands(ctx context.Context, lightList []Device, controlField LightControlField) []*cli.Command {
	return []*cli.Command{
		{
			Name:   "step-up",
			Usage:  "Increase brightness or temperature",
			Action: func(c *cli.Context) error { return adjustLightControlField(ctx, lightList, controlField, 10) },
		},
		{
			Name:   "step-down",
			Usage:  "Decrease brightness or temperature",
			Action: func(c *cli.Context) error { return adjustLightControlField(ctx, lightList, controlField, -10) },
		},
		{
			Name:  "get",
			Usage: "Get brightness or temperature",
			Action: func(c *cli.Context) error {
				val, err := getLightControlField(c.Context, lightList, controlField)
				if err != nil {
					return err
				}

				fmt.Printf("%d\n", val)
				return nil
			},
		},
		{
			Name:   "set",
			Usage:  "Set brightness or temperature",
			Action: func(c *cli.Context) error { return setLightControlField(c, lightList, controlField) },
		},
	}
}

func adjustLightControlField(ctx context.Context, lightList []Device, controlField LightControlField, change int) error {
	value, err := getLightControlField(ctx, lightList, controlField)
	if err != nil {
		return err
	}

	value += change
	if value > 100 {
		value = 100
	} else if value < 0 {
		value = 0
	}

	return setLightControlFieldWithValue(ctx, lightList, controlField, value)
}

func setLightControlField(c *cli.Context, lightList []Device, controlField LightControlField) error {
	value, err := strconv.Atoi(c.Args().First())
	if err != nil {
		return err
	}

	return setLightControlFieldWithValue(c.Context, lightList, controlField, value)
}

func setLightControlFieldWithValue(ctx context.Context, lightList []Device, controlField LightControlField, value int) error {
	lgs, err := fetchLightGroups(ctx, lightList)
	if err != nil {
		return err
	}

	for device, lightGroup := range lgs {
		for _, light := range lightGroup.Lights {
			switch controlField {
			case ControlBrightness:
				light.Brightness = value
			case ControlTemperature:
				light.Temperature = value
			}
		}

		logrus.Debug("Updating light group for ", device.GetDNSAddr())
		_, err = device.UpdateLightGroup(ctx, lightGroup)
		if err != nil {
			return err
		}
	}

	return nil
}

func getLightControlField(ctx context.Context, lightList []Device, controlField LightControlField) (int, error) {
	lgs, err := fetchLightGroups(ctx, lightList)
	if err != nil {
		return 0, err
	}

	for _, lightGroup := range lgs {
		for _, light := range lightGroup.Lights {
			switch controlField {
			case ControlBrightness:
				return light.Brightness, nil
			case ControlTemperature:
				return light.Temperature, nil
			}
		}
	}

	return 0, nil
}

func getDeviceStatus(ctx context.Context, lightList []Device) (string, error) {
	var sb strings.Builder

	for _, device := range lightList {
		logrus.Debug("Fetching device info for ", device.GetDNSAddr())
		deviceInfo, err := device.FetchDeviceInfo(ctx)
		if err != nil {
			return "", err
		}

		logrus.Debug("Fetching device settings for ", device.GetDNSAddr())
		deviceSettings, err := device.FetchSettings(ctx)
		if err != nil {
			return "", err
		}

		logrus.Debug("Fetching light group for ", device.GetDNSAddr())
		lightGroup, err := device.FetchLightGroup(ctx)
		if err != nil {
			return "", err
		}

		sb.WriteString(DeviceString(device, *deviceInfo, *deviceSettings, *lightGroup))
	}

	return sb.String(), nil
}
