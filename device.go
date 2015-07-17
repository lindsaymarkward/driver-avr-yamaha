package main

import (
	"math"

	"github.com/lindsaymarkward/go-avr-yamaha"
	"github.com/lindsaymarkward/go-ninja/devices"
	"github.com/ninjasphere/go-ninja/channels"
	"github.com/ninjasphere/go-ninja/model"
)

type Device struct {
	devices.MediaPlayerDevice
	avr *avryamaha.AVR
}

// makeNewDevice creates a Ninja Sphere Media Player device and
// sets all of the functions to handle events for play/pause/volume/power...
func makeNewDevice(driver *Driver, cfg *AVRConfig) (*Device, error) {
	log.Infof("Making new device for %v AVR with serial number %v at IP %v\n", cfg.Model, cfg.ID, cfg.IP)

	player, err := devices.CreateMediaPlayerDevice(driver, &model.Device{
		NaturalID:     cfg.ID, // serial number
		NaturalIDType: "yamaha-avr",
		Name:          &cfg.Name,
		Signatures: &map[string]string{
			"ninja:manufacturer": "Yamaha",
			"ninja:productName":  "Yamaha " + cfg.Model,
			"ninja:productType":  "MediaPlayer",
			"ninja:thingType":    "mediaplayer",
			"ip:serial":          cfg.ID,
		},
	}, driver.Conn)

	if err != nil {
		return nil, err
	}

	avr := avryamaha.AVR{IP: cfg.IP}
	// no need to set serial (ID) & name as this is just so we can access the avryamah (YNC) library
	// those are all stored in the driver config

	player.ApplyIsOn = func() (bool, error) {
		return avr.GetPower(cfg.Zone)
	}

	player.ApplyGetPower = func() (bool, error) {
		return avr.GetPower(cfg.Zone)
	}

	// Volume Channel
	player.ApplyVolumeUp = func() error {
		err := avr.ChangeVolume(cfg.VolumeIncrement, cfg.Zone)
		if err != nil {
			return err
		}
		newVolume, getError := avr.GetVolume(cfg.Zone)
		if getError == nil {
			player.UpdateVolumeState(&channels.VolumeState{
				Level: &newVolume, // float64
			})
		}
		return getError
	}

	player.ApplyVolumeDown = func() error {
		err := avr.ChangeVolume(-cfg.VolumeIncrement, cfg.Zone)
		if err != nil {
			return err
		}
		newVolume, getError := avr.GetVolume(cfg.Zone)
		if getError == nil {
			player.UpdateVolumeState(&channels.VolumeState{
				Level: &newVolume, // float64
			})
		}
		return getError
	}

	player.ApplyVolume = func(state *channels.VolumeState) error {
		// translate volume in range 0-1 to Min-Max
		// on my RX-V671 AVR, zone 2, min volume is -805 (-80.5 dB), max is 165 (+16.5 dB)
		value := *state.Level
		if value < 0 {
			value = 0
			state.Level = &value
		} else if value > 1 {
			value = 1
			state.Level = &value
		}

		volumeRange := cfg.MaxVolume - avryamaha.MinVolume
		volume := (value * volumeRange) + avryamaha.MinVolume
		// clamp volume to multiples of 0.5 to match AVR requirements
		volumeValue := int(conformToClosest(volume, 0.5) * 10)
		//		log.Infof("volumeRange %v, volume %v, volumeValue %v\n", volumeRange, volume, volumeValue)
		err := avr.SetVolume(volumeValue, cfg.Zone)
		if err != nil {
			return err // ?? an err here crashes the driver (does it still?). Perhaps we can make it more robust
		}
		player.UpdateVolumeState(state)
		return nil
	}

	player.ApplyToggleMuted = func() error {
		state, err := avr.ToggleMuted(cfg.Zone)
		player.UpdateVolumeState(&channels.VolumeState{Muted: &state})
		return err
	}

	// enable the volume channel, supporting mute (parameter is true)
	if err := player.EnableVolumeChannel(true); err != nil {
		player.Log().Errorf("Failed to enable volume channel: %s", err)
	}

	// on-off channel methods
	player.ApplyOff = func() error {
		player.UpdateOnOffState(false)
		return avr.SetPower("Standby", cfg.Zone)
	}

	// TODO: replace "Standby" etc. with a boolean (3 places)

	player.ApplyOn = func() error {
		player.UpdateOnOffState(true)
		return avr.SetPower("On", cfg.Zone)
	}

	player.ApplyToggleOnOff = func() error {
		state, err := avr.TogglePower(cfg.Zone)
		player.UpdateOnOffState(state)
		return err
	}

	// Workaround for on/off control mimicked by play/pause
	player.UpdatePowerPlay = func(state bool) {
		if state {
			player.UpdateControlState(channels.MediaControlEventPlaying)
		} else {
			player.UpdateControlState(channels.MediaControlEventPaused)
		}

	}

	// I can't find anywhere that the on/off states ever get set - on the sphereamid or in the app
	if err := player.EnableOnOffChannel("state"); err != nil {
		player.Log().Errorf("Failed to enable on-off channel: %s", err)
	}

	// NOTE: this is a workaround to get on/off when dragging to on/play or off/pause. Find a better way if possible
	// https://discuss.ninjablocks.com/t/mediaplayer-device-drivers/3776/2 (question asked)
	player.ApplyPlayPause = func(isPlay bool) error {
		if isPlay {
			player.UpdateControlState(channels.MediaControlEventPlaying)
			return player.ApplyOn()
		} else {
			player.UpdateControlState(channels.MediaControlEventPaused)
			return player.ApplyOff()
		}
	}

	if err := player.EnableControlChannel([]string{}); err != nil {
		player.Log().Errorf("Failed to enable control channel: %s", err)
	}

	return &Device{*player, &avr}, nil
}

func roundPlaces(f float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return round(f*shift) / shift
}

func round(f float64) float64 {
	return math.Floor(f + .5)
}

func conformToClosest(value, step float64) float64 {
	multiples := int(value / step)
	newValue := float64(multiples) * step
	return roundPlaces(newValue, 2)
}
