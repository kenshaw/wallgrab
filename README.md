# wallgrab

A quick util to grab Apple's wallpapers.

## Using

```sh
# list available wallpapers
$ wallgrab --list

# show available wallpapers using terminal graphics
$ wallgrab --show

# grab
$ wallgrab --grab

# grab and write playlist
$ wallgrab --grab --dest /path/to/wallpapers --m3u apple.m3u

# use with mpvpaper
$ mpvpaper -o 'no-audio --loop-playlist shuffle --speed=0.2' '*' /path/to/wallpapers/apple.m3u
```

### Sway

Example [sway](https://swaywm.org) config:

```gitconfig
# set up some variables
set {
  # window modifier key
  $mod Mod4

  # control path for mpv
  $mpvctl $HOME/.local/lib/mpvpaper/control

  # sets the output for the media title (read from the m3u) to the bottom right
  $mpvopt no-audio \
    --input-ipc-server=$mpvctl \
    --loop-playlist shuffle \
    --speed=0.8 \
    --osd-playing-msg='\${osd-ass-cc/0}{\\\\\\\\\\\\\\\\an3}\${osd-ass-cc/1}\${media-title}'
}

# run mpvpaper as wallpaper
exec_always {
  killall -9 mpvpaper
  killall -9 swaybg
  mpvpaper -o "$mpvopt" '*' $HOME/Pictures/backgrounds/apple/wallpapers.m3u
}

# bind modifier key + media keys to change/pause background
bindsym {
 $mod+XF86AudioStop exec socat - $mpvctl <<< 'cycle pause'
 $mod+XF86AudioPrev exec socat - $mpvctl <<< 'playlist-prev'
 $mod+XF86AudioPlay exec socat - $mpvctl <<< 'cycle pause'
 $mod+XF86AudioNext exec socat - $mpvctl <<< 'playlist-next'
}
```

> **Note:**
>
> The above \\'s are not a typo: due to the way escaping works with sway
> config, and in turn with mpvpaper, this is the correct number of \\'s.

To use with `swaylock-plugin`, [see the lock script here][shell-config-script].

### Notes

Quick commands:

```sh
# display text in bottom right corner
$ socat - $mpvctl <<< 'show-text ${osd-ass-cc/0}{\\an3}${osd-ass-cc/1}${media-title}'
```

> **Note:**
>
> ${osd-ass-cc/0} and ${osd-ass-cc/1} - starts and ends subtitle escaping
>
> \an<pos> - uses numpad numbers for location, hence 3 == lower right

- See [the mpv.io manual][mpvio] for command line switch information
- See [here][mpvprops] for available mpv text properties
- See [aegisub manual][aegisub] for more info on subtitle tags

[mpvio]: https://mpv.io/manual/stable/
[mpvprops]: https://mpv.io/manual/stable/#properties
[aegisub]: https://aegisub.org/docs/latest/ass_tags/
[shell-config-script]: https://github.com/kenshaw/shell-config/tree/master/sway/lock.sh
