package main

import (
	"fmt"
	"time"

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

func (c *Config) get(id string) *AVRConfig {
	for _, avr := range c.AVRs {
		if avr.ID == id {
			return avr
		}
	}
	return nil
}

type AVRConfig struct {
	ync.AVR                 // IP, ID, Name
	VolumeIncrement float64 `json:"volumeIncrement,string,omitempty"`
	MaxVolume       float64 `json:"maxVolume,string,omitempty"`
	Zone            int     `json:"zone,string,omitempty"`
	UpdateInterval  int     `json:"updateInterval,string,omitempty"`
}

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

func (d *Driver) Start(config *Config) error {
	log.Infof("Driver Starting with config %+v", config)

	if config.AVRs == nil {
		config.AVRs = make(map[string]*AVRConfig)
	}

	d.config = *config
	//	d.config = &Config{}

	for _, cfg := range config.AVRs {
		d.createAVRDevice(cfg)
	}

	d.Conn.MustExportService(&configService{d}, "$driver/"+info.ID+"/configure", &model.ServiceAnnouncement{
		Schema: "/protocol/configuration",
	})

	return nil
}

func (d *Driver) UpdateStates(device *Device, config *AVRConfig) {
	// set current volume & mute state
	volume, _ := device.avr.GetVolume(config.Zone)
	mute, _ := device.avr.GetMuted(config.Zone)
	// convert YNC volume value to float in range 0-1
	volumeRange := config.MaxVolume - ync.MinVolume
	volumeFloat := (volume - ync.MinVolume) / volumeRange
	device.UpdateVolumeState(&channels.VolumeState{Level: &volumeFloat, Muted: &mute})

	// set current power state
	power, _ := device.avr.GetPower(config.Zone)
	device.UpdateOnOffState(power)
}

func (d *Driver) createAVRDevice(config *AVRConfig) {

	device, err := newDevice(d, config)

	if config.UpdateInterval == 0 {
		config.UpdateInterval = defaultUpdateInterval
	}
	// regular updates to sync states so Ninja sees updates made to AVR externally
	go func() {
		for {
			d.UpdateStates(device, config)
			time.Sleep(time.Duration(config.UpdateInterval) * time.Second)
		}
	}()

	if err != nil {
		log.Errorf("Failed to create new Yamaha AVR device IP:%s ID:%s name:%s ", config.IP, config.ID, config.Name, err)
	} else {
		d.devices[config.ID] = device
		log.Infof("Created device at IP %v\n", device.avr.IP)
	}
}

// saveAVR saves configuration set in configuration form (Labs)
func (d *Driver) saveAVR(avr AVRConfig) error {

	// reset config
	//	d.config = Config{}
	//	return d.SendEvent("config", d.config)

	//	if !(&ync.AVR{IP: avr.IP}).Online(time.Second * 5) {
	//		return fmt.Errorf("Could not connect to TV. Is it online?")
	//	}
	// TODO: check with get command?

	//	mac, err := getMACAddress(avr.Host, time.Second*10)
	//	if err != nil {
	//		return fmt.Errorf("Failed to get mac address for TV. Is it online?")
	//	}
	serialNumber := "033E2543" // TODO: change to get from device, but needs to be unique... serial # available -UDP & HTTP (http://192.168.1.221:49154/MediaRenderer/desc.xml)?

	existing := d.config.get(serialNumber)
	// get returns a pointer, so existing refers to actual config device, not a copy
	if existing != nil {
		log.Infof("Re-ceating previously stored AVR, %v (%v)\n", avr.ID, existing.ID)
		existing.IP = avr.IP
		existing.Name = avr.Name
		existing.Zone = avr.Zone
		existing.VolumeIncrement = avr.VolumeIncrement
		// TODO: check/fix MaxVolume - clamp
		existing.MaxVolume = avr.MaxVolume
		device, ok := d.devices[serialNumber]
		if ok {
			device.avr.IP = avr.IP
		}
	} else {
		// new AVR - first-time setup
		log.Infof("First time - new AVR")
		avr.ID = serialNumber // ?? see above, get from XML
		d.config.AVRs[serialNumber] = &avr

		go d.createAVRDevice(&avr)
	}
	// ** temp
	fmt.Printf("Config now: %v\n", d.config)
	return d.SendEvent("config", d.config)
}

func (d *Driver) deleteAVR(id string) error {
	delete(d.config.AVRs, id)

	err := d.SendEvent("config", &d.config)

	return err
}
