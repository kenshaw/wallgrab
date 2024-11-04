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
$ wallgrab --grab --dest /path/to/wallpapers --playlist apple.m3u

# use with mpvpaper
$ mpvpaper -o 'no-audio --loop-playlist shuffle --speed=0.2' '*' /path/to/wallpapers/apple.m3u
```
