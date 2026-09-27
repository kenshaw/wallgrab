# Plan

This file holds the plan for wallgrab, every decision that shapes it, and the
questions that are open for Ken. wallgrab is small, so its decisions stay in
this file and are not split into one file each (dbmeta D111).

## Decisions

The entries are append only. A decision is never edited to change its
conclusion. When a later decision changes one, the older entry keeps its text,
and its heading names the decision that changed it. An amendment is visible
from both sides: if D10 amends D8, then the heading of D8 says "Amended by D10".

Each heading has a status after the title. The status starts with `Decided`
or `Proposed`. `Decided` means that the history shows that Ken chose it.
`Proposed` means that the history does not show it, and Ken must answer the
matching open question at the end of this file. `TestTheDecisionIndexIsComplete`,
`TestAnAmendmentPointsBothWays` and `TestEveryProposedDecisionHasAQuestion`
hold the table, the headings and the questions together.

D1 to D25 were written on 2026-09-27 from the history of the repository, from
`53a9487` (2024-11-04) to `93d60eb` (2026-09-26), and from the code at
`93d60eb`. Each entry names its commits. The reason comes from a commit
message, a code comment or the README. If none of these gives a reason, the
entry says "Reason not recorded".

A bare number such as D3 is a decision in this file. A decision of another
repository names that repository, such as dbmeta D110.

| Decision | Title | Status |
| --- | --- | --- |
| [D1](#d1-wallgrab-reads-the-manifest-that-apples-screen-saver-reads-decided) | wallgrab reads the manifest that Apple's screen saver reads | Decided |
| [D2](#d2-the-manifest-is-decoded-strictly-decided) | The manifest is decoded strictly | Decided |
| [D3](#d3-responses-are-cached-on-disk-and-the-wallpapers-are-not-decided) | Responses are cached on disk, and the wallpapers are not | Decided |
| [D4](#d4-grab-downloads-a-wallpaper-only-when-the-local-size-differs-decided) | grab downloads a wallpaper only when the local size differs | Decided |
| [D5](#d5-grab-can-write-an-m3u-playlist-with-durations-from-ffprobe-decided) | grab can write an m3u playlist, with durations from ffprobe | Decided |
| [D6](#d6-the-command-line-uses-xoox-and-each-action-is-a-subcommand-decided) | The command line uses xo/ox, and each action is a subcommand | Decided |
| [D7](#d7-the-dependencies-are-kept-at-their-newest-release-decided) | The dependencies are kept at their newest release | Decided |
| [D8](#d8-apples-hosts-are-verified-with-an-embedded-apple-root-ca-bundle-decided-amended-by-d10) | Apple's hosts are verified with an embedded Apple root CA bundle | Decided. Amended by D10 |
| [D9](#d9-a-name-that-repeats-gets-its-shot-id-and-a-dynamic-wallpaper-gets-its-orientation-decided) | A name that repeats gets its shot ID, and a dynamic wallpaper gets its orientation | Decided |
| [D10](#d10-the-ca-bundle-is-downloaded-at-run-time-and-cached-for-a-week-decided-amends-d8) | The CA bundle is downloaded at run time and cached for a week | Decided. Amends D8 |
| [D11](#d11-a-corrupt-cache-entry-is-deleted-and-read-once-more-decided) | A corrupt cache entry is deleted and read once more | Decided |
| [D12](#d12-every-text-that-a-person-reads-is-in-plain-english-decided) | Every text that a person reads is in plain English | Decided |
| [D13](#d13-ci-builds-vets-and-tests-on-linux-macos-and-windows-decided) | CI builds, vets and tests on Linux, macOS and Windows | Decided |
| [D14](#d14-the-skills-are-under-agents-and-claudeskills-links-to-them-decided-amended-by-d26) | The skills are under .agents, and .claude/skills links to them | Decided. Amended by D26 |
| [D15](#d15-wallgrab-reads-both-layouts-of-the-resources-tar-decided) | wallgrab reads both layouts of the resources tar | Decided |
| [D16](#d16-a-category-with-no-localized-string-takes-its-name-from-its-key-decided) | A category with no localized string takes its name from its key | Decided |
| [D17](#d17---os-accepts-macos-and-tvos-and-both-read-one-set-decided) | --os accepts macos and tvos, and both read one set | Decided |
| [D18](#d18-the-releases-are-a-table-in-the-source-decided-amended-by-d20) | The releases are a table in the source | Decided. Amended by D20 |
| [D19](#d19-history-holds-the-list-of-each-release-by-capture-date-decided) | history/ holds the list of each release, by capture date | Decided |
| [D20](#d20-the-os-versions-come-from-apple-and-the-newest-set-is-the-default-decided-amends-d18) | The OS versions come from Apple, and the newest set is the default | Decided. Amends D18 |
| [D21](#d21---version-accepts-a-codename-held-in-the-source-decided-amended-by-d22) | --version accepts a codename, held in the source | Decided. Amended by D22 |
| [D22](#d22-the-codenames-come-from-endoflifedate-decided-amends-d21) | The codenames come from endoflife.date | Decided. Amends D21 |
| [D23](#d23-wallgrab-is-released-by-tag-and-installs-from-the-latest-tag-decided) | wallgrab is released by tag, and installs from the latest tag | Decided |
| [D24](#d24-a-flag-can-be-renamed-or-removed-with-no-alias-proposed) | A flag can be renamed or removed with no alias | Proposed |
| [D25](#d25-wallgrab-is-one-file-maingo-in-package-main-proposed) | wallgrab is one file, main.go, in package main | Proposed |
| [D26](#d26-wallgrab-is-set-up-for-coding-agents-as-every-xo-repository-is-decided-amends-d14) | wallgrab is set up for coding agents as every xo repository is | Decided. Amends D14 |

## Purpose

wallgrab downloads the Apple Aerial wallpapers. These are the videos that
macOS and tvOS play as a screen saver. It lists them in any language that
Apple provides names for, draws a thumbnail of each one in a terminal that
supports graphics, and writes a playlist for a video player such as
`mpvpaper`.

## Current state

This is the state at `93d60eb`, which is tagged `v0.1.0`:

- The command is one file, `main.go`, of 1,418 lines. It has four
  subcommands: `list`, `show`, `grab` and `versions`.
- It reads the macOS releases v14, v15, v26 and v27. Apple gives names in 39
  to 44 languages, by release.
- It reads the OS versions from Apple, and the codenames from endoflife.date,
  so a new macOS release needs no code change (D20, D22).
- `history/` holds the output of `wallgrab list --sizes` for each release, by
  capture date (D19).
- Before D26, it had no tests. `go test ./...` reported no test files. The
  tests that D26 added check the layout of the repository. No test checks
  what the command does. [BACKLOG.md](BACKLOG.md) holds that as B11.
- CI builds, vets and tests on Linux, macOS and Windows (D13). The repository
  has no `.golangci.yml`.

## Direction

The commit messages give one direction: a new macOS release must work with no
code change. D20 and D22 did this for the versions and the codenames. D17
keeps `--os` so that a tvOS set can be added if Apple publishes one.

No other direction is recorded. The known work is in
[BACKLOG.md](BACKLOG.md).

## The decisions

### D1. wallgrab reads the manifest that Apple's screen saver reads. Decided.

Commits: `53a9487` (2024-11-04).

wallgrab reads a configuration plist from
`configuration.apple.com/configurations/internetservices/aerials/`. The plist
names a resources tar, and the tar holds `entries.json`, which lists every
wallpaper. macOS reads the same files. So wallgrab lists every wallpaper that
Apple publishes, and needs no list of its own.

Reason: the README says that wallgrab "reads the same manifest that macOS
reads, so it lists every wallpaper that Apple publishes".

### D2. The manifest is decoded strictly. Decided.

Commits: `53a9487`, `5073918` (2025-07-30), `6338914` (2026-09-13).

`getEntries` decodes `entries.json` with `DisallowUnknownFields`. A field that
Apple adds makes the decode fail. The first commit did this, and when Apple
added fields for v26 and v27, `5073918` and `6338914` added the fields to
`Asset` and `Subcategory` and kept the strict decode.

Reason not recorded. The effect is that a change to the manifest stops
wallgrab, and is not ignored.

### D3. Responses are cached on disk, and the wallpapers are not. Decided.

Commits: `53a9487`, `62e1ca8` (2026-09-26), `a7fed88` (2026-09-26).

Every request except the download of a wallpaper goes through
`github.com/kenshaw/diskcache`, in `~/.cache/wallgrab`. The cache keeps a
response for 30 days, and for 7 days when its content type is in the list in
`newDiskCache`. `62e1ca8` and `a7fed88` added the content types of the CA
bundle and the version feeds, so that they refresh weekly. The download of a
wallpaper uses `client(ctx, false)`, which does not cache.

Reason: the README says that wallgrab caches "the files that it needs to find
the wallpapers" and "does not cache the wallpapers themselves". The v27 set at
`history/20260926/v27.txt` is 74.47 GiB, and one wallpaper is up to 981 MiB.

### D4. grab downloads a wallpaper only when the local size differs. Decided.

Commits: `53a9487`.

`setDL` compares the size of the local file with the size from a HEAD request.
If the file is absent, or the two sizes differ, `grab` downloads it. A second
run of `grab` downloads only what changed.

Reason not recorded.

### D5. grab can write an m3u playlist, with durations from ffprobe. Decided.

Commits: `24d039b` (2024-11-05), `9390517` (2024-11-05).

`--m3u` names a playlist that `grab` writes in `--dest`. Each entry has the
duration that `ffprobe` reads from the file. If `ffprobe` is not on the path,
the duration is -1, which m3u reads as unknown. The name must be a file in
`--dest`, and a path that leaves it is an error.

Reason: the README uses the playlist with `mpvpaper`, and shows the title of
each entry on the screen.

### D6. The command line uses xo/ox, and each action is a subcommand. Decided.

Commits: `7644f63` (2024-11-27), `e7d0538` (2025-01-11).

`7644f63` replaced `spf13/cobra` with `github.com/xo/ox`. The `--list`,
`--show` and `--grab` flags became the `list`, `show` and `grab` subcommands.
The flags are the fields of `Args`, and each `ox` tag holds the help text.
`e7d0538` changed the size format to `ox.Size`.

Reason not recorded.

### D7. The dependencies are kept at their newest release. Decided.

Commits: 10 commits named "Updating dependencies" and 3 that update `xo/ox`,
from `527b298` (2024-11-25) to `61f6f90` (2026-02-27), and `6338914`.

Ken updates every dependency to its newest release, often. `go.mod` declares
`go 1.27`, and CI uses the stable release of Go.

Reason not recorded.

### D8. Apple's hosts are verified with an embedded Apple root CA bundle. Decided. Amended by D10.

Commits: `72b0f5f` (2025-09-14).

Before this commit, a request to Apple used `InsecureSkipVerify`. `72b0f5f`
embedded `apple_ca_bundle.pem`, added it to the system pool, and verified
every request. `update-apple-ca.sh` refreshed the file.

Reason: a comment at `93d60eb` says that "Apple's configuration and asset
hosts do not use a publicly trusted root". D10 moved the bundle out of the
binary.

### D9. A name that repeats gets its shot ID, and a dynamic wallpaper gets its orientation. Decided.

Commits: `6338914` (2026-09-13).

A dynamic wallpaper has one name for two orientations, so `getEntries` adds
the orientation to the name. Some languages give two wallpapers the same
name, so `getEntries` adds the shot ID to each name that repeats. A name
that still repeats is an error.

Reason: a comment says that Arabic and Slovenian translate "Hong Kong Skyline"
and "Hong Kong Horizon" the same way. The file name comes from the name, so
two wallpapers with one name write one file.

### D10. The CA bundle is downloaded at run time and cached for a week. Decided. Amends D8.

Commits: `62e1ca8` (2026-09-26).

wallgrab downloads Apple's root CA bundle from `tls-inspector/rootca` and
caches it for a week. The request uses the system roots. This commit removed
the embedded bundle and `update-apple-ca.sh`. The URL is the copy in the
repository, not the release asset.

Reason: the commit message says that "the bundle no longer goes stale between
releases". A comment says that the release asset redirects to a signed URL
that expires within the hour, so nothing can cache it.

### D11. A corrupt cache entry is deleted and read once more. Decided.

Commits: `62e1ca8`.

If a cached file cannot be read, wallgrab deletes that one entry and
downloads it again, once. `--clear` deletes the whole cache directory before
the command runs.

Reason: the commit message says that this "fixes the EOF failure reported on
the Aerials gist".

### D12. Every text that a person reads is in plain English. Decided.

Commits: `62e1ca8`.

The comments, the README and the strings that the user sees were rewritten
in plain English. The same commit added the `simple-english` skill. D26 makes
the skill a standing rule.

Reason not recorded in this repository. dbmeta D110 records the same rule for
every `xo` repository.

### D13. CI builds, vets and tests on Linux, macOS and Windows. Decided.

Commits: `62e1ca8`.

`.github/workflows/test.yml` runs `go build ./...`, `go vet ./...` and
`go test -v ./...` on `ubuntu-latest`, `macos-latest` and `windows-latest`,
with the stable release of Go, on every push and pull request.

Reason not recorded.

### D14. The skills are under .agents, and .claude/skills links to them. Decided. Amended by D26.

Commits: `62e1ca8`.

This commit added `go-pedantry` and `simple-english` as folders in
`.agents/skills`, and `.claude/skills/<name>` as symbolic links to them. It
also added `/skills-lock.json` to `.gitignore`, so the file that names the
source of each skill was not committed.

Reason not recorded. D26 replaced the links with copies and committed
`skills-lock.json`.

### D15. wallgrab reads both layouts of the resources tar. Decided.

Commits: `5073918` (2025-07-30), `6338914`, `177248e` (2026-09-26).

Apple changed the layout of the tar at v26. Up to v15, the names are one
plist per language, under `TVIdleScreenStrings.bundle`. From v26, they are
one binary table that holds every language. `5073918` and `6338914` read the
v26 layout alone. `177248e` reads the tar once into `Resources`, and picks the
layout from what the tar holds, not from the version. It also skips the
AppleDouble files that v26 ships.

Reason: the commit message says that "a release that keeps either layout
works". It was tested against 332 combinations of release, operating system
and language.

### D16. A category with no localized string takes its name from its key. Decided.

Commits: `ce72f24` (2025-07-30), `177248e`.

`ce72f24` fixed one Tahoe category with a check on its ID. `177248e` removed
that check, and `categoryName` now cuts a known prefix off the key when the
language has no string for it.

Reason: the commit message says that macOS v26 "ships
AerialSubcategoryDescriptionMac with no string, which showed the key to the
reader".

### D17. --os accepts macos and tvos, and both read one set. Decided.

Commits: `177248e`.

`--os` accepts `macos` and `tvos`. Apple publishes one set for both, so both
read the macOS configuration.

Reason: the README says that "the option exists so that a tvOS set can be
added if Apple ever separates them".

### D18. The releases are a table in the source. Decided. Amended by D20.

Commits: `5073918`, `6338914`, `177248e`, `c57a972` (2026-09-26), `29732e9`
(2026-09-26).

The release that `--version` names was held in the source. `5073918` made the
default `v26.0`, and `6338914` made it `v27.0`. `177248e` added a table of
releases. `c57a972` renamed `--macos-version` to `--version`. `29732e9`
added the `versions` command, which read the table, and accepted `27` and
`27.0` as well as `v27.0`.

Reason: `c57a972` says that `--os` now chooses the operating system, "so the
macos prefix no longer describes it". D20 replaced the table.

### D19. history/ holds the list of each release, by capture date. Decided.

Commits: `8f1e717` (2026-09-26), `ccf22d8` (2026-09-26).

`history/<date>/v<major>.txt` holds the output of `wallgrab list --sizes` for
one release on one day.

Reason: the commit messages say that the files let the sets "be compared side
by side", and that "a listing is a snapshot of what Apple published on the
day it was taken".

### D20. The OS versions come from Apple, and the newest set is the default. Decided. Amends D18.

Commits: `2b003e5` (2026-09-26).

wallgrab reads the OS versions from Apple's device management feed at
`gdmf.apple.com/v2/pmv`. `--version` has no default. Apple lists a version
before it publishes the wallpapers for it, so wallgrab walks back from the
newest version until a set answers. A configuration that answers 404 means
that Apple publishes no wallpapers for that version.

Reason: the commit message says that "a new macOS release needed an edit"
before this change.

### D21. --version accepts a codename, held in the source. Decided. Amended by D22.

Commits: `e7aa90b` (2026-09-26).

`--version` accepts a codename such as `tahoe` or `golden gate`, and ignores
its case and spacing. `versions` shows the codename of each release. The
codenames were held in the source.

Reason: the commit message says that Apple publishes no feed that names a
release. D22 found one outside Apple.

### D22. The codenames come from endoflife.date. Decided. Amends D21.

Commits: `a7fed88` (2026-09-26).

wallgrab reads the codenames from `endoflife.date/api/macos.json`. A release
that the feed does not name still works by number. If the feed cannot be
read, wallgrab logs it and continues with no codenames.

Reason: the commit message says that the codenames in the source needed an
edit for each release. The feed is 5 KB, and the macadmins SOFA feed is
300 KB and joins the name to the number in one string.

### D23. wallgrab is released by tag, and installs from the latest tag. Decided.

Commits: `93d60eb` (2026-09-26), tagged `v0.1.0`.

The README installs with `go install github.com/kenshaw/wallgrab@latest`, and
its header links to the releases page.

Reason: the commit message says that the install line "named the master
branch, which does not exist".

### D24. A flag can be renamed or removed with no alias. Proposed.

Commits: `7644f63`, `62e1ca8`, `c57a972`.

Three commits changed a flag and kept no alias for the old one. `7644f63`
replaced `--list`, `--show` and `--grab` with subcommands. `62e1ca8` removed
`--quiet`. `c57a972` renamed `--macos-version` to `--version`.

The history shows the three changes. It does not show that Ken chose this as
a rule. Open question 1 asks.

### D25. wallgrab is one file, main.go, in package main. Proposed.

Commits: every commit since `53a9487`.

All of the code is in `main.go`, in package `main`, and it grew from 680
lines to 1,418 lines. No commit split it.

The history shows the layout. It does not show that Ken chose it as a rule.
Open question 2 asks.

### D26. wallgrab is set up for coding agents as every xo repository is. Decided. Amends D14.

Decided: 2026-09-27, when Ken asked for this setup here. dbmeta D110 is the
standard, and dbmeta D111 says where the decisions of a small project go.
wallgrab is in `kenshaw`, not in `xo`, and follows the same standard.

- `AGENTS.md` holds the rules for a coding agent, and opens with the three
  standing rules of dbmeta D110.
- `CLAUDE.md` holds one line, `@AGENTS.md`. It is a file, not a symbolic link.
- `CONTRIBUTING.md` is new, and holds the commands that install the skills.
- `simple-english` and `go-pedantry` are copies in `.agents/skills/<name>` and
  in `.claude/skills/<name>`. The symbolic links of D14 are gone, because a
  Windows checkout writes a link as a small text file, and Claude Code then
  loads no skill and reports nothing. CI runs on Windows (D13).
- `skills-lock.json` is committed. `.gitignore` no longer ignores it, and it
  ignores `.claude/settings.local.json`.
- `.gitattributes` holds `* text=auto eol=lf`, so the two copies of a skill are
  the same bytes on every checkout.
- This file holds the plan and the decisions, and
  [BACKLOG.md](BACKLOG.md) holds the known work.

These tests hold the setup: `TestSkillsAreCopies`, `TestClaudeImportsAgents`,
`TestTheRootHoldsFourDocuments`, `TestTheDecisionIndexIsComplete`,
`TestAnAmendmentPointsBothWays` and `TestEveryProposedDecisionHasAQuestion`.
They use only the standard library.

## Open questions for Ken

1. D24: do you want the rule that a flag can be renamed or removed with no
   alias? tblfmt D37 says that no `xo` project promises backward
   compatibility. If yes, D24 becomes Decided. If no, write the rule for an
   alias.
2. D25: must wallgrab stay one file? `main.go` is 1,418 lines. If yes, D25
   becomes Decided. If no, say where it splits.
3. The repository has no `.golangci.yml`, and dburl and tblfmt have one. Do
   you want one here, and is it the tblfmt configuration?
4. B2 and B3 in [BACKLOG.md](BACKLOG.md) are faults in `grab` that can lose a
   download with no error. Do you want them fixed before the next tag?
