# footprint

Generate a per-repository report of a user's commits and squash-merged pull requests in a GitHub organization for a given month.

## Installation

Grab a static binary from [releases page](https://github.com/alternateved/footprint/releases) and install it in your `$PATH`.

Or install it with Go:

```sh
go install github.com/alternateved/footprint@latest
```

Or build it from source:

```sh
git clone https://github.com/alternateved/footprint
cd footprint
make install
```

## Authentication

`footprint` requires a GitHub personal access token with `repo` scope when accessing private repositories. It resolves one from the following sources, in order:

1. `GH_TOKEN` environment variable takes precedence.

```sh
export GH_TOKEN=ghp_xxxx
```

2. `gh` - if the [GitHub CLI](https://cli.github.com) is installed and authenticated, `footprint` uses its token automatically.

## Usage

```sh
footprint -u <USER> -o <ORG> [-y <YEAR>] [-m <MONTH>]
```
