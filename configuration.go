package main

import (
	"encoding/json"
	"fmt"

	"strconv"

	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/go-ninja/suit"
)

// TODO: optional force input on ON/Play
// TODO: make desired inputs a config item? (ideally, get from device, but doesn't seem possible?)
var inputs = []string{"NET RADIO", "AUDIO1", "AUDIO2", "USB", "TUNER"}

type configService struct {
	driver *Driver
}

func (c *configService) GetActions(request *model.ConfigurationRequest) (*[]suit.ReplyAction, error) {
	return &[]suit.ReplyAction{
		suit.ReplyAction{
			Name:        "",
			Label:       "Yamaha AV Receivers",
			DisplayIcon: "music",
		},
	}, nil
}

func (c *configService) Configure(request *model.ConfigurationRequest) (*suit.ConfigurationScreen, error) {
	log.Infof("Incoming configuration request. Action:%s Data:%s", request.Action, string(request.Data))

	switch request.Action {
	case "list":
		return c.list()
	case "":
		// present the list or new AVR screen
		if len(c.driver.config.AVRs) > 0 {
			return c.list()
		}
		fallthrough
	case "new":
		return c.edit(AVRConfig{})

	case "edit":
		var values map[string]string
		err := json.Unmarshal(request.Data, &values)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal save config request %s: %s", request.Data, err))
		}

		config := c.driver.config.get(values["avr"])

		if config == nil {
			return c.error(fmt.Sprintf("Could not find AVR with id: %s", values["avr"]))
		}
		return c.edit(*config)

	case "delete":
		var values map[string]string
		err := json.Unmarshal(request.Data, &values)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal save config request %s: %s", request.Data, err))
		}

		err = c.driver.deleteAVR(values["avr"])
		if err != nil {
			return c.error(fmt.Sprintf("Failed to delete AVR: %s", err))
		}

		return c.list()

	case "save":
		var cfg AVRConfig
		err := json.Unmarshal(request.Data, &cfg)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal save config request %s: %s", request.Data, err))
		}

		err = c.driver.saveAVR(cfg)
		if err != nil {
			return c.error(fmt.Sprintf("Could not save AVR: %s", err))
		}

		return c.list()

	case "toggleOnOff":
		var cfg AVRConfig
		err := json.Unmarshal(request.Data, &cfg)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal turnOn config request %s: %s", request.Data, err))
		}
		// turn on/off (which updates state)
		c.driver.devices[cfg.ID].ToggleOnOff()
		return c.list()

	case "turnOn":
		var cfg AVRConfig
		err := json.Unmarshal(request.Data, &cfg)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal turnOn config request %s: %s", request.Data, err))
		}
		c.driver.devices[cfg.ID].SetOnOff(true)
		return c.control(c.driver.config.AVRs[cfg.ID])

	case "turnOff":
		var cfg AVRConfig
		err := json.Unmarshal(request.Data, &cfg)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal turnOn config request %s: %s", request.Data, err))
		}
		c.driver.devices[cfg.ID].SetOnOff(false)
		return c.control(c.driver.config.AVRs[cfg.ID])

	case "control":
		var cfg AVRConfig
		err := json.Unmarshal(request.Data, &cfg)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal control config request %s: %s", request.Data, err))
		}
		return c.control(c.driver.config.AVRs[cfg.ID])

	case "input":
		var values map[string]string
		err := json.Unmarshal(request.Data, &values)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal input config request %s: %s", request.Data, err))
		}
		c.driver.config.AVRs[values["ID"]].SetInput(values["input"], c.driver.config.AVRs[values["ID"]].Zone)
		// send/save config
		//		c.driver.config.AVRs[values["ID"]].Input =
		//		c.driver.SendEvent("config", c.driver.config)
		return c.control(c.driver.config.AVRs[values["ID"]])

	case "zone":
		var values map[string]string
		err := json.Unmarshal(request.Data, &values)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal input config request %s: %s", request.Data, err))
		}
		zoneNumber, _ := strconv.Atoi(values["zone"])
		log.Infof("\nzone - %v\n", zoneNumber)
		// send/save config
		c.driver.config.AVRs[values["ID"]].Zone = zoneNumber
		c.driver.SendEvent("config", c.driver.config)
		return c.control(c.driver.config.AVRs[values["ID"]])

	case "confirmDelete":
		var values map[string]string
		err := json.Unmarshal(request.Data, &values)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal save config request %s: %s", request.Data, err))
		}
		return c.confirmDelete(values["avr"])
	default:
		return c.error(fmt.Sprintf("Unknown action: %s", request.Action))
	}
}

func (c *configService) error(message string) (*suit.ConfigurationScreen, error) {
	return &suit.ConfigurationScreen{
		Sections: []suit.Section{
			suit.Section{
				Contents: []suit.Typed{
					suit.Alert{
						Title:        "Error",
						Subtitle:     message,
						DisplayClass: "danger",
					},
				},
			},
		},
		Actions: []suit.Typed{
			suit.ReplyAction{
				Label: "Back",
				Name:  "list",
			},
		},
	}, nil
}

func (c *configService) control(avr *AVRConfig) (*suit.ConfigurationScreen, error) {
	var inputActions []suit.ActionListOption
	mainTitle := "Main"
	// if zone has not been set, make it the default (main zone)
	if avr.Zone == 0 {
		avr.Zone = 1
	}
	if avr.Zone == 1 {
		mainTitle += " *"
	}
	zoneActions := []suit.ActionListOption{suit.ActionListOption{
		Title: mainTitle,
		Value: "1",
	}}
	// TODO: (one day if needed), show inputs relevant to current zone (e.g. Zone 2 has no HDMI)
	// create input actions
	currentInput, _ := avr.GetInput(avr.Zone)
	for _, input := range inputs {
		selected := ""
		if input == currentInput {
			selected = " *"
		}
		inputActions = append(inputActions, suit.ActionListOption{
			Title: input + selected,
			Value: input,
		})
	}
	// create zone actions (main already defined)
	for i := 2; i < avr.Zones+1; i++ {
		selected := ""
		if i == avr.Zone {
			selected = " *"
		}
		zoneActions = append(zoneActions, suit.ActionListOption{
			Title: "Zone " + fmt.Sprintf("%v", i) + selected,
			Value: fmt.Sprintf("%v", i),
		})
	}

	screen := suit.ConfigurationScreen{
		Title: "Control " + avr.Name,
		Sections: []suit.Section{
			suit.Section{
				Title: "Select Zone",
				Contents: []suit.Typed{
					suit.InputHidden{
						Name:  "ID",
						Value: avr.ID,
					},
					suit.ActionList{
						Name:    "zone",
						Options: zoneActions,
						PrimaryAction: &suit.ReplyAction{
							Name:        "zone",
							DisplayIcon: "home",
						},
					},
				},
			},
			// TODO: (one day) only show if power is on; input selection doesn't work if power is off
			suit.Section{
				Title:    "Select Input - Zone " + fmt.Sprintf("%v", avr.Zone),
				Subtitle: "Input selection doesn't work unless the zone's power is on.",
				Contents: []suit.Typed{
					suit.InputHidden{
						Name:  "ID",
						Value: avr.ID,
					},
					suit.ActionList{
						Name:    "input",
						Options: inputActions,
						PrimaryAction: &suit.ReplyAction{
							Name:        "input",
							DisplayIcon: "arrow-circle-right",
						},
					},
				},
			},
			suit.Section{
				Title: "Power - Zone " + fmt.Sprintf("%v", avr.Zone),
				Contents: []suit.Typed{
					//					suit.InputHidden{
					//						Name:  "ID",
					//						Value: avr.ID,
					//					},
					suit.ActionList{
						Name:    "ID",
						Options: []suit.ActionListOption{suit.ActionListOption{Title: "Turn On", Value: avr.ID}},
						PrimaryAction: suit.ReplyAction{
							Name:        "turnOn",
							Label:       "Turn On",
							DisplayIcon: "power-off",
						},
						SecondaryAction: suit.ReplyAction{
							Name:         "turnOff",
							Label:        "Turn Off",
							DisplayIcon:  "power-off",
							DisplayClass: "danger",
						},
					},
				},
			},
		},
		Actions: []suit.Typed{
			suit.ReplyAction{
				Label: "Back",
				Name:  "list",
			},
		},
	}
	return &screen, nil
}

func (c *configService) list() (*suit.ConfigurationScreen, error) {

	var avrs []suit.ActionListOption
	var avrActions []suit.ActionListOption

	for _, avr := range c.driver.config.AVRs {
		// create edit actions
		avrs = append(avrs, suit.ActionListOption{
			Title: avr.Name + " (" + avr.Model + ")",
			Value: avr.ID,
		})
		// create power actions
		title := avr.Name
		if isOn, _ := c.driver.devices[avr.ID].IsOn(); isOn {
			title += " (On) - Turn Off"
		} else {
			title += " (Off) - Turn On"
		}
		avrActions = append(avrActions, suit.ActionListOption{
			Title: title,
			Value: avr.ID,
		})
	}

	screen := suit.ConfigurationScreen{
		Title: "Yamaha AV Receivers",
		Sections: []suit.Section{
			suit.Section{
				Title: "Edit",
				Contents: []suit.Typed{
					suit.ActionList{
						Name:    "avr",
						Options: avrs,
						PrimaryAction: &suit.ReplyAction{
							Name:        "edit",
							DisplayIcon: "pencil",
						},
						SecondaryAction: &suit.ReplyAction{
							Name:         "confirmDelete",
							Label:        "Delete",
							DisplayIcon:  "trash",
							DisplayClass: "danger",
						},
					},
				},
			},
			suit.Section{
				Title: "Control",
				Contents: []suit.Typed{
					suit.ActionList{
						Name:    "ID",
						Options: avrActions,
						PrimaryAction: &suit.ReplyAction{
							Name:        "toggleOnOff",
							Label:       "Turn On",
							DisplayIcon: "power-off",
						},
						SecondaryAction: &suit.ReplyAction{
							Name:        "control",
							Label:       "Control",
							DisplayIcon: "sliders",
						},
					},
				},
			},
		},
		Actions: []suit.Typed{
			suit.CloseAction{
				Label: "Close",
			},
			suit.ReplyAction{
				Label:        "New AVR",
				Name:         "new",
				DisplayClass: "success",
				DisplayIcon:  "star",
			},
		},
	}

	return &screen, nil
}

func (c *configService) edit(config AVRConfig) (*suit.ConfigurationScreen, error) {

	title := "New Yamaha AVR"
	if config.ID != "" {
		title = "Editing Yamaha AVR (" + config.Model + ")"
	}

	screen := suit.ConfigurationScreen{
		Title:    title,
		Subtitle: "Please complete all of the fields.",
		Sections: []suit.Section{
			suit.Section{
				Contents: []suit.Typed{
					suit.InputHidden{
						Name:  "id",
						Value: config.ID,
					},
					suit.InputText{
						Name:        "name",
						Before:      "Name",
						Placeholder: "Preferred name",
						Value:       config.Name,
					},
					suit.InputText{
						Name:        "ip",
						Before:      "IP",
						Placeholder: "IP address",
						Value:       config.IP,
					},
					// TODO: Consider if I can check # zones (can't just query amp?)
					suit.InputText{
						Name:        "zones",
						Before:      "Zones",
						Placeholder: "Number of zones (1, 2, ...)",
						Value:       config.Zones,
					},
					suit.InputText{
						Name:        "maxVolume",
						Before:      "Max Volume",
						Placeholder: "Use multiples of 0.5",
						Value:       config.MaxVolume,
					},
					suit.InputText{
						Name:        "updateInterval",
						Before:      "Update Interval",
						Placeholder: "in seconds",
						Value:       config.UpdateInterval,
					},
					// volume increment is only relevant/used if ApplyVolume is not defined
					// leave this code in, in case it's ever needed
					//					suit.RadioGroup{
					//						Name:     "volumeIncrement",
					//						Title:    "Volume Increment",
					//						Subtitle: "This has no effect unless you change the driver to not implement ApplyVolume",
					//						Value:    fmt.Sprintf("%0.1f", config.VolumeIncrement), // set selected radio to value in config
					//						Options: []suit.RadioGroupOption{
					//							suit.RadioGroupOption{
					//								Title: "0.5",
					//								Value: "0.5",
					//							},
					//							suit.RadioGroupOption{
					//								Title: "1",
					//								Value: "1.0",
					//							},
					//							suit.RadioGroupOption{
					//								Title: "2",
					//								Value: "2.0",
					//							},
					//							suit.RadioGroupOption{
					//								Title: "5",
					//								Value: "5.0",
					//							},
					//						},
					//					},
				},
			},
		},
		Actions: []suit.Typed{
			suit.ReplyAction{
				Label: "Cancel",
				Name:  "list",
			},
			suit.ReplyAction{
				Label:        "Save",
				Name:         "save",
				DisplayClass: "success",
				DisplayIcon:  "star",
			},
		},
	}

	return &screen, nil
}

// confirmDelete is a config screen for confirming/cancelling deleting of AVR
func (c *configService) confirmDelete(id string) (*suit.ConfigurationScreen, error) {
	return &suit.ConfigurationScreen{
		Sections: []suit.Section{
			suit.Section{
				Title: "Confirm Deletion of " + c.driver.config.AVRs[id].Name + " (" + c.driver.config.AVRs[id].Model + ")",
				Contents: []suit.Typed{
					suit.Alert{
						Title:        "Do you really want to delete this AV Receiver?",
						DisplayClass: "danger",
						DisplayIcon:  "warning",
					},
					suit.InputHidden{
						Name:  "avr",
						Value: id,
					},
				},
			},
		},
		Actions: []suit.Typed{
			suit.ReplyAction{
				Label:       "Cancel",
				Name:        "list",
				DisplayIcon: "close",
			},
			suit.ReplyAction{
				Label:        "Confirm - Delete",
				Name:         "delete",
				DisplayClass: "warning",
				DisplayIcon:  "check",
			},
		},
	}, nil
}
