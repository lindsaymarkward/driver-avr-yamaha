package main

import (
	"github.com/lindsaymarkward/go-ninja/devices"
	"github.com/lindsaymarkward/go-ync"
	"github.com/ninjasphere/go-ninja/channels"
	"github.com/ninjasphere/go-ninja/model"
)

type Device struct {
	devices.MediaPlayerDevice
	avr *ync.AVR
}

func newDevice(driver *Driver, cfg *AVRConfig) (*Device, error) {
	log.Infof("\nMaking new device with ID %v at IP %v\n\n", cfg.ID, cfg.IP)

	player, err := devices.CreateMediaPlayerDevice(driver, &model.Device{
		NaturalID:     cfg.ID, // serial number
		NaturalIDType: "yamaha-avr",
		Name:          &cfg.Name,
		Signatures: &map[string]string{
			"ninja:manufacturer": "Yamaha",
			"ninja:productName":  "Yamaha AVR", // TODO: add model number here (like RX-V671) cfg.Model
			"ninja:productType":  "MediaPlayer",
			"ninja:thingType":    "mediaplayer",
			"ip:serial":          cfg.ID,
		},
	}, driver.Conn)

	if err != nil {
		return nil, err
	}

	avr := ync.AVR{
		IP: cfg.IP,
		// serial & name ??
	}

	/*
		Next step... compare with FakeDriver & samsung-tv
	*/

	player.ApplyIsOn = func() (bool, error) {
		isOn, err := avr.GetPower(cfg.Zone)
		return isOn, err
	}

	// Volume Channel
	player.ApplyVolumeUp = func() error {
		return avr.ChangeVolume(1, cfg.Zone)
	}

	player.ApplyVolumeDown = func() error {
		return avr.ChangeVolume(-1, cfg.Zone)
	}

	player.ApplyToggleMuted = func() error {
		state, err := avr.ToggleMuted(cfg.Zone)
		player.UpdateVolumeState(&channels.VolumeState{Muted: &state})
		return err
	}

	// enable the volume channel, supporting mute (true parameter)
	if err := player.EnableVolumeChannel(true); err != nil {
		player.Log().Errorf("Failed to enable volume channel: %s", err)
	}

	// On-off Channel
	player.ApplyOff = func() error {
		player.UpdateOnOffState(false)
		return avr.SetPower("Standby", cfg.Zone)
	}

	player.ApplyOn = func() error {
		player.UpdateOnOffState(true)
		return avr.SetPower("On", cfg.Zone)
	}

	player.ApplyToggleOnOff = func() error {
		state, err := avr.TogglePower(cfg.Zone)
		player.UpdateOnOffState(state)
		return err
	}

	if err := player.EnableOnOffChannel("turnOff", "turnOn", "toggle"); err != nil {
		//	if err := player.EnableOnOffChannel("state"); err != nil {
		player.Log().Errorf("Failed to enable on-off channel: %s", err)
	}

	if err := player.EnableControlChannel([]string{}); err != nil {
		player.Log().Errorf("Failed to enable control channel: %s", err)
	}

	return &Device{*player, &avr}, nil
}
