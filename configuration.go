package main

import (
	"encoding/json"
	"fmt"

	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/go-ninja/suit"
)

type configService struct {
	driver *Driver
}

func (c *configService) GetActions(request *model.ConfigurationRequest) (*[]suit.ReplyAction, error) {
	return &[]suit.ReplyAction{
		suit.ReplyAction{
			Name:        "",
			Label:       "Yamaha AVRs",
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
			return c.error(fmt.Sprintf("Failed to unmarshal save config request %s: %s", request.Data, err))
		}
		c.driver.devices[cfg.ID].ApplyOn()
		return c.list()
	case "turnOff":
		var cfg AVRConfig
		err := json.Unmarshal(request.Data, &cfg)
		if err != nil {
			return c.error(fmt.Sprintf("Failed to unmarshal save config request %s: %s", request.Data, err))
		}
		fmt.Println(cfg)
		c.driver.devices[cfg.ID].ApplyOff()
		return c.list()
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

func (c *configService) list() (*suit.ConfigurationScreen, error) {

	var avrs []suit.ActionListOption
	var avrActions []suit.ActionListOption

	for _, avr := range c.driver.config.AVRs {
		// create edit actions
		avrs = append(avrs, suit.ActionListOption{
			Title: avr.Name,
			Value: avr.ID,
		})
		// create control actions
		title := avr.Name
		if isOn, _ := c.driver.devices[avr.ID].IsOn(); isOn {
			title += " *"
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
				Title:    "Control",
				Subtitle: "* indicates AVR is currently on",
				Contents: []suit.Typed{
					suit.StaticText{
						Value: "to be AVR name (loop ",
					},
					suit.ActionList{
						Name:    "ID",
						Options: avrActions,
						PrimaryAction: &suit.ReplyAction{
							Name:        "turnOn",
							Label:       "Turn On",
							DisplayIcon: "toggle-on",
						},
						SecondaryAction: &suit.ReplyAction{
							Name:        "turnOff",
							Label:       "Turn Off",
							DisplayIcon: "toggle-off",
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
					// TODO: Consider if I can check # zones (query amp) - OR (better) use select list
					suit.InputText{
						Name:        "zone",
						Before:      "Zone",
						Placeholder: "Zone number (1, 2, ...)",
						Value:       config.Zone,
					},
				},
			},
		},
		Actions: []suit.Typed{
			suit.CloseAction{
				Label: "Cancel",
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
						Title:        "Confirm Delete" + c.driver.config.AVRs[id].Name,
						Subtitle:     "Do you really want to delete this AVR?",
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
