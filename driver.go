package main

// sphere-yamaha Ninja Sphere driver for Yamaha AV Receivers
// using YNC (Yamaha Network Control) protocol from go-ync (https://github.com/lindsaymarkward/go-ync)
// Lindsay Ward, June 2015 - https://github.com/lindsaymarkward/sphere-yamaha

import (
	"time"

	"fmt"

	"github.com/lindsaymarkward/go-ync"
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/channels"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/go-ninja/support"
)

const defaultUpdateInterval = 5

var info = ninja.LoadModuleInfo("./package.json")
var log = logger.GetLogger(info.Name)

type Driver struct {
	support.DriverSupport
	config  Config
	devices map[string]*Device
}

type Config struct {
	AVRs map[string]*AVRConfig
}

// an AVRConfig stores details about an AV Receiver including reference to the ync library's AVR struct
type AVRConfig struct {
	ync.AVR                 // IP, ID, Name
	VolumeIncrement float64 `json:"volumeIncrement,string,omitempty"`
	MaxVolume       float64 `json:"maxVolume,string,omitempty"`
	Zones           int     `json:"zones,string,omitempty"`
	Zone            int     `json:"zone,string,omitempty"`
	UpdateInterval  int     `json:"updateInterval,string,omitempty"`
}

// NewDriver creates a new driver with an empty map of names
// initialises and exports Ninja stuff
func NewDriver() (*Driver, error) {
	driver := &Driver{
		devices: make(map[string]*Device),
	}

	err := driver.Init(info)
	if err != nil {
		log.Fatalf("Failed to initialize driver: %s", err)
	}

	err = driver.Export(driver)
	if err != nil {
		log.Fatalf("Failed to export driver: %s", err)
	}

	return driver, nil
}

// Start runs when the driver is started - called by the Ninja system (not the driver itself),
// creates devices for all AVRs in the config, exports the configuration service
func (d *Driver) Start(config *Config) error {
	log.Infof("Driver starting with config %+v", config)

	if config.AVRs == nil {
		config.AVRs = make(map[string]*AVRConfig)
	}

	d.config = *config

	for _, cfg := range config.AVRs {
		d.createAVRDevice(cfg)
	}

	d.Conn.MustExportService(&configService{d}, "$driver/"+info.ID+"/configure", &model.ServiceAnnouncement{
		Schema: "/protocol/configuration",
	})

	return nil
}

// UpdateStates updates all relevant states in the driver to account for external changes to the AVR
func (d *Driver) UpdateStates(device *Device, config *AVRConfig) error {
	// set current states
	state, err := device.avr.GetState(config.Zone)
	if err != nil {
		return err
	}
	// convert YNC volume value to float in range 0-1
	volumeRange := config.MaxVolume - ync.MinVolume
	volumeFloat := (state.Volume - ync.MinVolume) / volumeRange

	device.UpdateVolumeState(&channels.VolumeState{Level: &volumeFloat, Muted: &state.Muted})
	device.UpdateOnOffState(state.Power)
	return nil
}

// createAVRDevice makes a new device from the config details passed in,
// starts a function that regularly updates the driver states
func (d *Driver) createAVRDevice(config *AVRConfig) error {

	device, err := makeNewDevice(d, config)
	if err != nil {
		errorMsg := fmt.Errorf("Failed to create new Yamaha AVR device IP:%s ID:%s name:%s - %s", config.IP, config.ID, config.Name, err)
		log.Errorf(fmt.Sprintf("%s", errorMsg))
		return errorMsg
	}

	if config.UpdateInterval == 0 {
		config.UpdateInterval = defaultUpdateInterval
	}
	// regular updates to sync states so Ninja sees updates made to AVR externally
	go func() {
		for {
			if device != nil {
				d.UpdateStates(device, config)
			}
			time.Sleep(time.Duration(config.UpdateInterval) * time.Second)
		}
	}()

	d.devices[config.ID] = device
	log.Infof("Created device with ID %v at IP %v\n", config.ID, device.avr.IP)
	return nil
}

// saveAVR saves configuration set in configuration form (Labs)
func (d *Driver) saveAVR(avr AVRConfig) error {
	// read data from the amp's XML details using IP to see if it's online
	err := avr.GetXMLData()
	if err != nil {
		errorMsg := fmt.Errorf("Could not connect to AVR (%v). Is it online?\n", err)
		log.Errorf(fmt.Sprintf("%s", errorMsg))
		// NOTE: could consider saving the config anyway (temp, don't "sendevent")
		// so you don't have to re-enter everything, but seems we can't since we don't have an ID
		return errorMsg
	}
	log.Infof("Got model: %v, at IP: %v\n", avr.Model, avr.ID)

	// if AVR already exists in config, just update config; otherwise, create new device
	_, ok := d.config.AVRs[avr.ID]
	if ok {
		// NOTE: here is where we could handle multiple devices for one AVR - multiple zones
		// use a config option, check it here - if it's wanted, use serial number + zone as key
		d.config.AVRs[avr.ID] = &avr
	} else {
		// new AVR - first-time setup, create device
		if err = d.createAVRDevice(&avr); err != nil {
			return err
		}
		d.config.AVRs[avr.ID] = &avr
	}
	return d.SendEvent("config", d.config)
}

// deleteAVR deletes an AVR from the config map
// NOTE: we can't yet unexport a device, so...?
func (d *Driver) deleteAVR(id string) error {
	delete(d.config.AVRs, id)
	// not sure about deleting devices - doesn't actually delete the device unless we restart the driver...
	delete(d.devices, id)

	err := d.SendEvent("config", &d.config)

	return err
}
