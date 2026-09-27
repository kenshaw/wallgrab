# Backlog

This file holds the work that is known and not done. A decision about an item
goes in [PLAN.md](PLAN.md). When an item is done, remove it from this file and
name it in the commit message.

B1 to B11 were collected on 2026-09-27. The line numbers are at `93d60eb`,
and they can move. On that day, GitHub showed no issue and no pull request for
`kenshaw/wallgrab`, open or closed. Each item names where it came from:

- TODO: a TODO comment in the code.
- Read: a fault found when reading the code. No program ran it.

## Faults in grab

### B1. The duration of each file is read after every download ends.

- Source: TODO, at `main.go:259-260`, from `9390517` (2024-11-05).
- Text: `move ffprobe duration read into actual asset read, and put as part of
  workload`.
- `doGrab` calls `addDur` after `getAssets` returns. `addDur` runs `ffprobe`
  on each file, one at a time. The TODO asks to run `ffprobe` in the task that
  downloads the file.

### B2. An error in a download or a size request is lost.

- Source: Read, at `main.go:303` and `main.go:374`.
- `getSizes` and `getAssets` give each task to `pool.SubmitErr`, and drop the
  `pond.Task` that it returns. `pool.StopAndWait` returns no error. So an
  error in a task is lost, and both funcs always return nil. `grab` reports
  success when a download fails.

### B3. On four CPUs or fewer, grab starts every download at once.

- Source: Read, at `main.go:52-57`.
- `main` sets `Streams` to 8 or 6 when the machine has more than 4 CPUs.
  Otherwise `Streams` stays 0. `pond` reads a limit of 0 as no limit
  (`pool.go:175` in `pond/v2` v2.7.1), so all 164 downloads of v27 start at
  the same time.
- Origin: `5073918` (2025-07-30) removed the default of 4 that `53a9487` set.

### B4. The size request does not check the HTTP status.

- Source: Read, at `main.go:729-747`.
- `getSize` returns `res.ContentLength` for any status. An error page gives
  the size of the page, and a response with no length gives -1. `setDL`
  rejects only a size of 0.

### B5. A failed download leaves an empty file or an error page.

- Source: Read, at `main.go:380-422`.
- `getAssets` opens the output file with `O_TRUNC` before it sends the
  request, and it does not check the status of the response. A failed request
  leaves an empty file, and an error status writes the error page to the file.
  The next `grab` downloads the file again, because the size differs (D4).

## Other faults

### B6. show does not close a preview image that decodes.

- Source: Read, at `main.go:222-235`.
- `doShow` closes the body only when `image.Decode` fails. Each preview that
  decodes leaves its body open until the command ends.

### B7. getAssets computes a total that nothing reads.

- Source: Read, at `main.go:352-358`.
- The loop adds to `total`, and nothing reads it. The width `n` starts from
  the first asset even when that asset is not downloaded, and the loop skips
  the first asset.

### B8. writeM3U drops the errors of each write.

- Source: Read, at `main.go:766-772`.
- `writeM3U` returns the error of `f.Close` only. It drops the error of each
  `fmt.Fprintln` and `fmt.Fprintf`. If a write fails, the playlist is cut
  short, and `grab` reports no error.

### B9. A comment names the wrong release for the change of layout.

- Source: Read, at `main.go:1411-1412`.
- The comment on `loctableName` says "Before macOS v27". The layout changed
  at v26 (D15), as the README and the other comments say.

### B10. getSize sets the User-Agent header twice.

- Source: Read, at `main.go:740`.
- `newReq` already sets the header. The second `req.Header.Set` has no effect
  and can go.

## Tests

### B11. No test checks what the command does.

- Source: Read. Before D26, `go test ./...` reported no test files.
- `matchLang`, `parseMajor`, `foldName`, `matchCodename`, `matchOS`,
  `configURL`, `classify`, `categoryName` and `expand` take no network and no
  disk. They can have table tests. B2 to B5 can have a test with an
  `httptest.Server`.
