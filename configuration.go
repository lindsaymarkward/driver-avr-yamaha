package main

// TODO: add config for #zones then create option buttons (in same page as inputs) for each (main, 2...) that set current zone. Setting zone saves the config - same actions but will call different zone
// TODO: optional force input on ON/Play

import (
	"encoding/json"
	"fmt"

	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/go-ninja/suit"
)

// TODO: make desired inputs a config item?
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

	case "turnOn":
		var cfg AVRConfig
		err := json.Unmarshal(request.Data, &cfg)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal turnOn config request %s: %s", request.Data, err))
		}
		// turn on/off (which updates state)
		c.driver.devices[cfg.ID].ToggleOnOff()
		return c.list()

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
		//		fmt.Println(values)
		//		c.driver.devices[values["ID"]].SetInput(values["input"])
		c.driver.config.AVRs[values["ID"]].SetInput(values["input"], c.driver.config.AVRs[values["ID"]].Zone)
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
				Label: "Cancel",
				Name:  "list",
			},
		},
	}, nil
}

func (c *configService) control(avr *AVRConfig) (*suit.ConfigurationScreen, error) {
	var inputActions []suit.ActionListOption
	// create input actions
	currentInput, err := avr.GetInput(avr.Zone)
	log.Infof("Current input is %v %v", currentInput, err)
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

	screen := suit.ConfigurationScreen{
		Title: "Control " + avr.Name,
		Sections: []suit.Section{
			suit.Section{
				Title: "Select Input",
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
		},
		Actions: []suit.Typed{
			suit.ReplyAction{
				Label: "Cancel",
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
			Title: avr.Name,
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
							Name:        "turnOn",
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
		title = "Editing Yamaha AVR"
	}

	screen := suit.ConfigurationScreen{
		Title: title,
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
					// TODO: Consider if I can check # zones (can't just query amp) - use select list
					suit.InputText{
						Name:        "zone",
						Before:      "Zone",
						Placeholder: "Zone number (1, 2, ...)",
						Value:       config.Zone,
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
					suit.RadioGroup{
						Name:     "volumeIncrement",
						Title:    "Volume Increment",
						Subtitle: "This has no effect unless you change the driver to not implement ApplyVolume",
						Value:    fmt.Sprintf("%0.1f", config.VolumeIncrement), // set selected radio to value in config
						Options: []suit.RadioGroupOption{
							suit.RadioGroupOption{
								Title: "0.5",
								Value: "0.5",
							},
							suit.RadioGroupOption{
								Title: "1",
								Value: "1.0",
							},
							suit.RadioGroupOption{
								Title: "2",
								Value: "2.0",
							},
							suit.RadioGroupOption{
								Title: "5",
								Value: "5.0",
							},
						},
					},
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
				Contents: []suit.Typed{
					suit.Alert{
						Title:        "Confirm deletion of " + c.driver.config.AVRs[id].Name,
						Subtitle:     "Do you really want to delete this AV Receiver?",
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
