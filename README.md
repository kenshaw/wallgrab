# wallgrab

<div align="center">
  <a href="#installing" title="Installing">Installing</a> |
  <a href="#using" title="Using">Using</a> |
  <a href="#releases" title="Releases">Releases</a> |
  <a href="#the-cache" title="The cache">The cache</a> |
  <a href="#sway" title="Sway">Sway</a> |
  <a href="#notes" title="Notes">Notes</a> |
  <a href="https://github.com/kenshaw/wallgrab/issues" title="Issues">Issues</a>
</div>

<br/>

`wallgrab` downloads Apple Aerial wallpapers. These are the videos that macOS
and tvOS play as a screen saver.

`wallgrab` reads the same manifest that macOS reads, so it lists every
wallpaper that Apple publishes, in any of the [44 languages](#using) that Apple
provides names for. It can [write a playlist](#using) for a video player, and
it can [draw a thumbnail of each wallpaper](#using) in a terminal that supports
graphics.

[![Unit Tests][wallgrab-ci-status]][wallgrab-ci]
[![Go Reference][goref-wallgrab-status]][goref-wallgrab]
[![Discord Discussion][discord-status]][discord]

[wallgrab-ci]: https://github.com/kenshaw/wallgrab/actions/workflows/test.yml "Test CI"
[wallgrab-ci-status]: https://github.com/kenshaw/wallgrab/actions/workflows/test.yml/badge.svg "Test CI"
[goref-wallgrab]: https://pkg.go.dev/github.com/kenshaw/wallgrab "Go Reference"
[goref-wallgrab-status]: https://pkg.go.dev/badge/github.com/kenshaw/wallgrab.svg "Go Reference"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"

## Installing

Install in the usual Go fashion:

```sh
$ go install github.com/kenshaw/wallgrab@master
```

## Using

```sh
# list the available wallpapers
$ wallgrab list

# list the wallpapers with the size of each one
$ wallgrab list --sizes

# list the names in another language
$ wallgrab list --lang ja

# list the OS versions that have wallpapers
$ wallgrab versions

# list the wallpapers of an older macOS release
$ wallgrab list --version v15.0

# draw a thumbnail of each wallpaper in the terminal
$ wallgrab show

# download the wallpapers
$ wallgrab grab

# download to a directory, and write a playlist
$ wallgrab grab --dest /path/to/wallpapers --m3u aerials.m3u

# use with mpvpaper
$ mpvpaper -o 'no-audio --loop-playlist shuffle --speed=0.2' '*' /path/to/wallpapers/aerials.m3u
```

Run `wallgrab list --lang xx` to see every language that Apple provides names
for. The command fails and lists them. See [Sway](#sway) for a desktop
configuration that plays the wallpapers, and [Notes](#notes) for the `mpv`
commands that control it.

### Releases

Apple publishes a separate set of wallpapers for each major OS release, and
keeps the older sets in place. wallgrab downloads the newest set by default.
Run `wallgrab versions` to list them all. `--version` chooses an older set,
and takes a number or a codename:

| Release | Codename | Wallpapers | Languages |
| --- | --- | --- | --- |
| `v14.0` | Sonoma | 134 | 39 |
| `v15.0` | Sequoia | 137 | 40 |
| `v26.0` | Tahoe | 156 | 43 |
| `v27.0` (newest) | Golden Gate | 164 | 44 |

`--version 27`, `--version 27.0`, `--version v27.0`, and
`--version "golden gate"` all mean the same release. A codename ignores case
and spacing, so `goldengate` works too.

wallgrab reads the list of OS versions from Apple's device management feed at
`https://gdmf.apple.com/v2/pmv`, so a new macOS release needs no change here.
Apple lists a version before it publishes the wallpapers for it, so wallgrab
walks back from the newest version until a set answers.

That feed carries version numbers alone. Apple publishes no feed that names a
release, so wallgrab reads the codenames from `https://endoflife.date/api/macos.json`.
A release that the codename feed does not name still works by number, and
`wallgrab versions` shows it with no codename.

Apple changed the layout of these files at v26. Up to v15 the names are one
plist per language, and from v26 they are one table that holds every language.
wallgrab reads both, so every release above works the same way.

`--os` accepts `macos` and `tvos`. Apple publishes one set of wallpapers for
both, so the two give the same result today. The option exists so that a tvOS
set can be added if Apple ever separates them.

### The cache

wallgrab keeps the files that it needs to find the wallpapers in
`~/.cache/wallgrab` for a week. It does not cache the wallpapers themselves.

When a cached file is truncated or corrupt, wallgrab deletes that one file and
downloads it again. To delete the whole cache directory instead, add `--clear`
to any command:

```sh
$ wallgrab list --clear
```

### Sway

Example [sway](https://swaywm.org) config:

```gitconfig
# set up some variables
set {
  # modifier keys (windows key)
  $mod Mod4
  $shf $mod+Shift

  # control path for mpv
  $mpvctl $HOME/.local/lib/mpvpaper/control

  # sets the output for the media title (read from the m3u) to the bottom right
  $mpvopt \
    --hwdec \
    --no-audio \
    --input-ipc-server=$mpvctl \
    --shuffle \
    --loop-playlist \
    --speed=0.8 \
    --osd-playing-msg='\${osd-ass-cc/0}{\\\\\\\\\\\\\\\\an3}\${osd-ass-cc/1}\${media-title}'

  # alternate location, bottom center with margin from bottom, stay on screen
  # for 7.5 seconds
  $mpvopt \
    --hwdec \
    --no-audio \
    --input-ipc-server=$mpvctl \
    --loop-playlist \
    --shuffle \
    --speed=0.8 \
    --osd-margin-y=70 \
    --osd-playing-msg-duration=7500 \
    --osd-playing-msg='\${osd-ass-cc/0}{\\\\\\\\\\\\\\\\an2}\${osd-ass-cc/1}\${media-title}'
}

# run mpvpaper as wallpaper
exec {
  mpvpaper -o "$mpvopt" '*' $HOME/Pictures/backgrounds/aerials/aerials.m3u
}

# bind modifier key + media keys to change/pause background
bindsym {
  $mod+XF86AudioStop exec socat - $mpvctl <<< 'cycle pause'
  $mod+XF86AudioPrev exec socat - $mpvctl <<< 'playlist-prev'
  $mod+XF86AudioPlay exec socat - $mpvctl <<< 'cycle pause'
  $mod+XF86AudioNext exec socat - $mpvctl <<< 'playlist-next'
  $shf+XF86AudioPlay exec socat - $mpvctl <<< 'show-text ${osd-ass-cc/0}{\\an2}${osd-ass-cc/1}${media-title} 7500'
}
```

> **Note:**
>
> The number of backslashes above is correct. Sway config and mpvpaper each
> remove one level of escaping.

To use with `swaylock-plugin`, [see the lock script here][shell-config-script].

### Notes

Quick commands:

```sh
$ export mpvctl=/path/to/control/socket

# display text in bottom right corner
$ socat - $mpvctl <<< 'show-text ${osd-ass-cc/0}{\\an3}${osd-ass-cc/1}${media-title}'

# set pause
$ socat - $mpvctl <<< 'set pause yes'

# cycle pause
$ socat - $mpvctl <<< 'cycle pause'

# get property
$ socat - $mpvctl <<< '{ "command": ["get_property", "pause"] }'

# list properties
$ mpv --list-properties
```

> **Note:**
>
> `${osd-ass-cc/0}` starts subtitle escaping. `${osd-ass-cc/1}` ends it.
>
> `\an<pos>` sets the position with numpad numbers. 3 is the lower right
> corner.

- See [the mpv.io manual][mpvio]
- See [mpv.io commands][mpvcommands] for commands that can be sent via the control socket
- See [mpv.io properties][mpvprops] for available mpv text properties
- See [aegisub manual][aegisub] for more info on subtitle tags
- See [Aerials discussion thread][aerialsgist]

[mpvio]: https://mpv.io/manual/stable/
[mpvprops]: https://mpv.io/manual/stable/#properties
[mpvcommands]: https://mpv.io/manual/stable/#list-of-input-commands
[aegisub]: https://aegisub.org/docs/latest/ass_tags/
[shell-config-script]: https://github.com/kenshaw/shell-config/tree/master/sway/lock.sh
[aerialsgist]: https://gist.github.com/theothernt/57a51cade0c12c407f48a5121e0939d5
