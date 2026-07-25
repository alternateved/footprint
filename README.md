# footprint

Generate a per-repository report of a user's commits and squash-merged pull requests in a GitHub organization for a given month.

## Example

```console
$ footprint -u fadenler -o morrico -y 2026 -m 6
Report for user 'fadenler' in organization 'morrico' (2026-06-01 - 2026-06-30)

Repository "daemon" contributions:
Rotate cache keys on config reload (b1aa2da)
Merge pull request #28 from morrico/remove-legacy-backend (f367579)

Repository "website" contributions:
#1842: Debounce search input on the main page (3a42f0b, 81ba8aa)
#1990: Add dark mode toggle to settings (9e42b14, 972494a, 700ea00)
```

## What counts as a contribution

`footprint` groups your month's work by repository and reports two kinds of
entries:

- **Squash-merged pull requests**: lines starting with `#<number>:`. The PR title is shown, followed by the short SHAs of the commits that made up the branch.
- **Plain commits**: everything else, shown as the commit subject and its short SHA. This covers direct pushes and merge commits that weren't squashed.

Entries are matched by author and committer date, so a commit lands in the report for the month it was committed, not the month its PR opened.

## Installation

Grab a static binary from [releases page](https://github.com/alternateved/footprint/releases) and install it in your `$PATH`.

With Go:

```sh
go install github.com/alternateved/footprint@latest
```

From source:

```sh
git clone https://github.com/alternateved/footprint
cd footprint
make install
```

## Authentication

`footprint` requires a GitHub personal access token with `repo` scope when accessing private repositories. It resolves one from the following sources, in order:

1. The `GH_TOKEN` environment variable.

```sh
export GH_TOKEN=ghp_xxxx
```

2. The [GitHub CLI](https://cli.github.com). If `gh` is installed and authenticated, `footprint` uses its token automatically.

## Usage

```sh
footprint -u <USER> -o <ORG> [-y <YEAR>] [-m <MONTH>]
```

Year and month default to the current date.
