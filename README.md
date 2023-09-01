# Snowy lab installer

```
#######################################################
#                                                     #
#   o-o                              o          o     #
#  |                                 |          |     #
#   o-o  o-o  o-o o   o   o o  o     |      oo  O-o   #
#      | |  | | |  \ / \ /  |  |     |     / |  |  |  #
#  o--o  o  o o-o   o   o   o--O     O---o o-o- o-o   #
#                              |                      #
#                           o--o                      #
#                                                     #
#######################################################
```

snowy-lab-installer is a dead-simple install wizard for Snowy lab NixOS. It's the fastest way to get from ISO to working installation.

From the NixOS installation USB/CD:

```
sudo nix-shell https://github.com/wexder/snowy-lab-installer/archive/main.tar.gz
```

## Development

In this directory run `servefile --tar --compression gzip --port 12345 .`. Then, while that's running `nix-shell -p ngrok --run "ngrok http 12345"`.

Now in your VM/device, run

```
nix-collect-garbage && sudo nix-shell http://blah-blah-blah.ngrok.io/snowy-lab-installer.tar.gz
```

You may need `sudo umount --lazy /mnt` periodically as well.

## Credits
Full credits to samuale from whom this script was forked https://github.com/samuela/nixos-up
