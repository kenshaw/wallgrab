# Contributing to wallgrab

This file is for a person who changes wallgrab. [AGENTS.md](AGENTS.md) holds
the same rules for a coding agent, with more detail.

## Building and testing

wallgrab is one Go command, in `main.go`. Before you send a change, run these
commands in the repository root. `gofmt -l .` must print nothing, and the
other two must pass:

```sh
gofmt -l .
go vet ./...
go test ./...
```

The tests check the layout of the repository: the agent skills, `CLAUDE.md`,
the documents in the root, and the decisions in `docs/PLAN.md`. They do not
use the network.

To try the command, run `go run . list`. It reads Apple's servers, so it needs
the network.

## Documents

- [README.md](README.md) tells a person how to use wallgrab.
- [AGENTS.md](AGENTS.md) holds the rules for a coding agent.
- [docs/PLAN.md](docs/PLAN.md) holds the plan, every decision and the open
  questions.
- [docs/BACKLOG.md](docs/BACKLOG.md) holds the work that is known and not done.

Only `README.md`, `AGENTS.md`, `CLAUDE.md` and this file belong in the
repository root. A new document goes in `docs/`.

A decision goes in `docs/PLAN.md`, with a row in the index at the top of that
file. If a decision changes an earlier one, both headings must name each
other. "Writing documentation" in `AGENTS.md` gives the form.

## Agent skills

The repository holds two agent skills. A skill is a set of instructions that a
coding agent loads for a task. `simple-english` sets how prose is written, and
`go-pedantry` sets how Go is written.

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and it writes each skill into two folders. Codex and
other agents read `.agents/skills/<name>`, and Claude Code reads
`.claude/skills/<name>`.

To install the skills again or to update them, run these commands in the
repository root:

```sh
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
npx skills@1.7.0 add oborchers/fractional-cto --skill go-pedantry --agent codex claude-code --copy -y
```

Keep `--copy`. Without it, the command writes `.claude/skills/<name>` as a
symbolic link. A Windows checkout writes a symbolic link as a small text file,
and Claude Code then loads no skill and reports nothing. `TestSkillsAreCopies`
fails on a link, on a missing copy, on two copies that differ, and on a skill
that `skills-lock.json` does not name.

`CLAUDE.md` holds one line, `@AGENTS.md`, which imports the rules in
`AGENTS.md`. Edit `AGENTS.md`, not `CLAUDE.md`.

`.claude/settings.local.json` holds the Claude Code permissions of one person.
The root `.gitignore` ignores it.
