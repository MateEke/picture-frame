---
title: Contributing
description: How the project is built, and how to set up a development environment.
---

Contributions are welcome. This page covers the stack and how to get a development environment
running. The repository's [`CONTRIBUTING.md`](https://github.com/MateEke/picture-frame/blob/main/CONTRIBUTING.md)
and [`STYLE.md`](https://github.com/MateEke/picture-frame/blob/main/STYLE.md) hold the full detail on
workflow and code style.

## The stack

The frame is one static **Go** binary with a **SvelteKit** front end, on Svelte 5, embedded in it.
The backend lives in `internal/`, the front end in `web/`, and the build compiles the front end
first and embeds it. There is no separate server or database. See
[The story & the hard parts](/development/story/) for why it is shaped this way.

## Getting set up

You need a recent Go and Node toolchain. From a clone of the repository:

```sh
go mod download
make hooks          # install the git hooks
cd web && npm install
```

`make watch` runs the development servers together, the Vite front end with the Go backend in
front of it, with all hardware mocked so it works on any machine.

## Building and testing

Run these from the repository root:

| Command         | Action                                                      |
| --------------- | ----------------------------------------------------------- |
| `make build`    | Build the complete binary, front end then backend.          |
| `make test`     | Run the Go and front-end unit tests with the coverage gate. |
| `make lint`     | Run all linters: Go, the front end, shell, and workflows.   |
| `make test-e2e` | Run the Playwright end-to-end suite against a real build.   |

Always build with `make build` rather than the backend alone, so the embedded front end matches
the binary.

## Conventions

Commits follow [Conventional Commits](https://www.conventionalcommits.org), which a git hook
checks. The full conventions, including the test and style rules, are in the repository's
[`CONTRIBUTING.md`](https://github.com/MateEke/picture-frame/blob/main/CONTRIBUTING.md) and
[`STYLE.md`](https://github.com/MateEke/picture-frame/blob/main/STYLE.md). The front-end specifics
live in [`web/README.md`](https://github.com/MateEke/picture-frame/blob/main/web/README.md).

## AI-assisted contributions

AI was used as a tool to build this project ([the story](/development/story/) has the why), so
AI-assisted contributions are welcome. They are held to the same bar as any other change: the pull
request stays a reviewable size, the change is properly tested, and it follows the style guide.
