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
$ mpvpaper -o 'no-audio --speed=0.1' '*' $HOME/Pictures/backgrounds/apple/dubai_3.mov
```
