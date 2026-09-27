# wallgrab

`wallgrab` is a command that downloads the Apple Aerial wallpapers, which are
the videos that macOS and tvOS play as a screen saver. It reads the manifest
that macOS reads (D1), lists the wallpapers in any language that Apple names
them in, draws a thumbnail of each in a terminal that supports graphics, and
writes an m3u playlist for a video player such as `mpvpaper` (D5).

## Standing rules

These hold in every `xo` repository, for every coding agent. dbmeta D110
records them, and D26 applies them here.

1. Stage changes for review. Commit and push only when Ken says so.
2. Load the `simple-english` skill before you write any text that a person
   reads: project documentation, a code comment, an error message or a
   commit message.
3. In a Go project, load the `go-pedantry` skill before you write or review
   Go code. A rule in this file wins where the two conflict.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Edit this file, not that one.

## Which document to read

| If you are | Read |
| --- | --- |
| using the command | [README.md](README.md) |
| asking why something is the way it is | the table at the top of [docs/PLAN.md](docs/PLAN.md) |
| about to decide something that is not written down | "Open questions for Ken" in [docs/PLAN.md](docs/PLAN.md). Ask Ken |
| looking for work that is known and not done | [docs/BACKLOG.md](docs/BACKLOG.md) |
| changing how a release or a version is found | D18, D20, D21 and D22 in [docs/PLAN.md](docs/PLAN.md) |
| changing how the tar or the manifest is read | D2, D15 and D16 in [docs/PLAN.md](docs/PLAN.md) |
| changing the cache or the certificates | D3, D8, D10 and D11 in [docs/PLAN.md](docs/PLAN.md) |
| changing `grab` or the playlist | D4 and D5 in [docs/PLAN.md](docs/PLAN.md), then B1 to B5 in [docs/BACKLOG.md](docs/BACKLOG.md) |
| adding, renaming or removing a flag | D6 and D24 in [docs/PLAN.md](docs/PLAN.md) |
| capturing the listing of a release | D19 in [docs/PLAN.md](docs/PLAN.md) |
| writing a document, a code comment, an error message or a commit message | the `simple-english` skill. Load it first. Then "Writing documentation" below |
| writing or reviewing Go code | the `go-pedantry` skill. Load it first. Then "Go conventions" below |
| adding or updating an agent skill | "Agent skills" in [CONTRIBUTING.md](CONTRIBUTING.md), and D26 |

`CONTRIBUTING.md` holds the same things for a person, and it is shorter.

A document that is not in that table does not exist. If you cannot find where
something is written down, it is not written down. Ask Ken. Do not decide it
yourself, and do not write it as though it were settled.

A decision named by a bare number, such as D3, is a decision of this
repository, in `docs/PLAN.md`. A decision of another repository names that
repository, such as dbmeta D110.

## Layout

- `main.go` holds all of the code, in package `main` (D25).
- `history/<date>/v<major>.txt` holds the output of `wallgrab list --sizes`
  for one release on one day (D19).
- `skills_test.go` and `docs_test.go` check the layout of the repository
  (D26). No test checks what the command does (B11).
- `docs/` holds every document except the four in the root.
- `.agents/skills` and `.claude/skills` hold the agent skills, as copies.
- `.github/workflows/test.yml` is the CI workflow (D13).

`.gitignore` names some scratch files that are in the author's checkout and
are not part of the project: `x.sh`, `crossfade.txt`, `mpv-live-filters/`
and `xfade-ffmpeg-script/`. Leave them as they are.

`main.go` is roughly in this order: `main` and `Args`, the four commands, the
download, the resources tar and the names, the HTTP client and the cache, the
releases, the manifest types, and the constants.

## Go conventions

These are the conventions that the code follows. Match the code around you.

- A command is a method of `Args`, named `do<Command>`. `main` registers it
  with `ox.Sub`. A flag is an exported field of `Args`, and its `ox` tag holds
  the help text in lower case (D6).
- `Args` keeps each result it reads from the network in an unexported field,
  such as `resURL`, `majors`, `codenames` and `loctable`. A getter returns
  that field when it is set, so that the network is read once for each run.
- `context.Context` is the first parameter, and it is named `ctx`.
- Every request goes through `args.newReq`, which sets the user agent, and
  `args.client(ctx, cache)`. A request that is not the download of a
  wallpaper passes `cache` as true (D3).
- A sentinel error is an unexported variable with the `err` prefix, made with
  `errors.New`: `errNotFound` and `errCorrupt`. Compare with `errors.Is`.
- A command wraps each step with `fmt.Errorf("unable to <verb> <noun>: %w",
  err)`. This differs from `go-pedantry`, which asks for a gerund, and the
  code wins. Always wrap with `%w`.
- An error string starts with a lower case letter, except where it starts
  with a name, such as "Apple publishes no aerial wallpapers". An error for a
  bad value names the values that are available, as `matchLang` and `matchOS`
  do.
- The receiver of `Args` is `args`, the receiver of `Entries` is `entries`,
  and the receiver of `Release` is `release`. The receiver of `Asset` is
  `a`. This differs from `go-pedantry`, and the code wins.
- Write `switch { case ...: }` in place of a chain of `if` and `else`, and
  `switch v, err := f(); {` to test a result and its error together.
- Sort map keys with `slices.Sorted(maps.Keys(m))`.
- A doc comment is a full sentence that starts with the name. A comment in a
  func body starts in lower case and says why.
- `args.logger` writes to stderr when `--verbose` is set, and it does nothing
  otherwise. Log each URL that a request reads.
- A URL or a name inside the tar is a constant at the end of `main.go`, with a
  comment that says what it is.

## Before you stage

Standing rule 1 applies. Stage the change with `git add`, and give Ken a
proposed commit message. Do not tag. These must pass first:

```sh
gofmt -l .
go vet ./...
go test ./...
```

`gofmt -l .` must print nothing. The repository has no `.golangci.yml`, so do
not run `golangci-lint` as a check. Open question 3 in `docs/PLAN.md` asks
Ken about it.

CI runs `go build ./...`, `go vet ./...` and `go test -v ./...` on Linux,
macOS and Windows, with the stable release of Go (D13).

The command reads Apple's servers, so a run of `wallgrab` needs the network.
To see a change, run `go run . list` or `go run . versions`. `grab` downloads
about 75 GiB for v27. No part of wallgrab needs a database, a container or a
virtual machine.

## Writing documentation

Only `README.md`, `AGENTS.md`, `CLAUDE.md` and `CONTRIBUTING.md` belong in the
repository root, and `TestTheRootHoldsFourDocuments` checks it. A new
document goes in `docs/`. Add it to the table at the top of this file and to
the list of documents in `README.md`.

A decision goes in `docs/PLAN.md` and nowhere else. Its heading is
`### D<n>. <Title>. <Status>.`, and the status starts with `Decided` or
`Proposed`. Write `Decided` only when Ken chose it. If a decision changes an
earlier one, write `Amends D<n>` in its status, and `Amended by D<m>` in the
status of the earlier one. Add a row to the index at the top of the file, with
the same title and status. `TestTheDecisionIndexIsComplete` and
`TestAnAmendmentPointsBothWays` check both.

Known work and known faults go in `docs/BACKLOG.md`, with the place that each
came from.

Standing rule 2 applies to every such text. Write short sentences in the
active voice, with no contractions, no semicolons and no em dashes. Use can,
will and must, and do not use should, may or might. Put the condition before
the command.
