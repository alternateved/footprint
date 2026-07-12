# footprint

Generate a per-repository report of a user's commits and squash-merged pull requests in a GitHub organization for a given month.

## Installation

```sh
go install github.com/alternateved/footprint@latest
```

## Authentication

`footprint` requires a GitHub personal access token with `repo` scope when accessing private repositories, provided via environment variable:

```sh
export GH_TOKEN=ghp_xxxx
```

A `.env` file is also supported.

## Usage

```sh
footprint -u <USER> -o <ORG> [-y <YEAR>] [-m <MONTH>]
```
