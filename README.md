# driver-avr-yamaha
Ninja Sphere driver (Go) for controlling Yamaha Audio Video Receivers (AVRs)

Allowing control of one zone at a time from the sphereamid and phone app

  - power  - tap sphereamid to toggle
  - volume - slider in app and airwheel gesture for sphereamid
  
Use the configuration (in Labs or http://ninjasphere.local) to:
 
  - create and edit an AVR (IP, name, maximum volume, update frequency)
  - control power
  - set zone 
  - set input/power for selected zone
  
Installation
------------

Copy both package.json and the binary (from the release) into `/data/sphere/user-autostart/drivers/driver-avr-yamaha` (create the directory as needed) and run `nservice driver-avr-yamaha start` on (or restart) the sphereamid.

Known Issues
------------

  - NOTE: There is no intention to make a full-featured "remote" of this with media controls and more.
  - The driver doesn't yet find Yamaha AVRs using SSDP. You have to enter your IP address.
  - On/off is handled using the play/pause actions as presented by Ninja. There doesn't seem to be a way to control on/off directly with the current "media-player" device type.
  - You can't have multiple devices with the same IP/ID. This is probably fine, but some people may want to have one device per zone (e.g. so you could have main in the TV room and zone 2 on the deck). Let me know if you want this - it could be done (with a config option that is checked when using the serial number as map key).
  - Inputs are currently limited. It doesn't check that the input is valid for the selected zone, and can't tell what inputs are actually available.

