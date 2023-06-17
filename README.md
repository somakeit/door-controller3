# door-controller3
A new door controller, because the members area is broken and nobody can fix it

This code allows the existing door-controller2 hardware work with Nottinghack's
HMS2 members area software.

## Required raspberry pi pins:
* 1  - MFRC522_3V3
* 6  - MFRC522_Ground
* 15 - Door strike/latch
* 16 - MFRC522_IRQ
* 18 - LED
* 19 - MFRC522_MOSI
* 21 - MFRC522_MISO
* 22 - MFRC522_RST
* 23 - MFRC522_SCK
* 24 - MFRC522_SDA

# Installation
1. Commission the raspberry pi (host) with Linux, set up SSH as desired, VDU as desired.
2. Make the above wiring.
3. Enable SPI and GPIO.
4. Connect the host with the HMS server, if they are not on the same LAN then this is *should* be done using an encrypted VPN tunnel, usually openvpn:
   1. Install openvpn on the host.
   2. Generate a new client key for the host on the HMS server, this is normally done using [openvpn-install](https://github.com/angristan/openvpn-install).
   3. The configuration generated is for full-tunnel but we need split-tunel, add `route-nopull` to the config file on a new line just before the certificates.
   4. Copy the generated openvpn config to the host at `/etc/openvpn/client.conf`, make sure the extension is `.conf`.
   5. Restart openvpn `sudo systemctl restart openvpn` (if this doesn't pick up the config just try a reboot).
   6. Test the connection to the database: `nc -zv 10.8.0.1 3306`.
5. Build the door controller (this repo):
   1. Install [Go](https://go.dev) on your machine.
   2. Run `make doord`.
6. Install the door controller:
   1. Create a user for doord to run as: `useradd -G spi,gpio doord`.
   2. Make the logging directory:
      ```sh
      mkdir /var/log/doord
      chown doord:doord /var/log/doord
      ```
   3. Copy `dist/etc/logrotate.d/doord` from this repo to `/etc/logrotate.d/doord` on the host.
   4. Disable login on tty1 because doord will use it: `systemctl mask getty@tty1.service`. If you want to log in on the console you can use ctrl+alt+F2 to use the next tty.
   5. Copy `dist/etc/systemd/system/doord.service` from this repo to `/etc/systemd/system/doord.service` on the host and edit the `-hms` argument inside it to be the correct DSN for the database, edit the `-door` and `-side` arguments to be the correct side of the correct door (these settings will eventually be in a proper config file, see [#1](https://github.com/somakeit/door-controller3/issues/1)).
   6. Copy `doord` to the host at `/usr/local/bin/doord`.
   7. Enable doord at boot: `sudo systemctl enable doord`
7. Reboot to stop tty1 login, start doord and make sure it does start on boot.
