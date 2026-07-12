# footprint

Generate a per-repository report of a user's commits and squash-merged pull requests in a GitHub organization for a given month.

## Installation

```sh
go install github.com/alternateved/footprint@latest
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
