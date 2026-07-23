package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/go-github/v89/github"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type config struct {
	user  string
	org   string
	since time.Time
	until time.Time
}

type report struct {
	name     string
	messages []string
}

func main() {
	config := initializeFlags()

	ghToken := resolveToken()
	client, err := github.NewClient(github.WithAuthToken(ghToken))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Report for user '%s' in organization '%s' (%s - %s)\n\n",
		config.user,
		config.org,
		config.since.Format(time.DateOnly),
		config.until.Format(time.DateOnly),
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	byRepo, err := fetchCommitsByRepo(ctx, client, config)
	if err != nil {
		log.Fatal(err)
	}

	reports := buildReports(ctx, client, config.org, byRepo)
	for _, report := range reports {
		printReport(report)
	}
}

func initializeFlags() config {
	user := flag.String("u", "", "GitHub username")
	org := flag.String("o", "", "GitHub organization")
	year := flag.Int("y", time.Now().Year(), "year")
	month := flag.Int("m", int(time.Now().Month()), "month")
	help := flag.Bool("h", false, "print footprint's help")
	version := flag.Bool("v", false, "print footprint's version")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), `footprint - report a user's monthly contributions in an org

Usage:
  footprint -u <USER> -o <ORG> [-y <YEAR>] [-m <MONTH>]

Authentication:
  A GitHub token is resolved from, in order:
    1. GH_TOKEN environment variable
    2. gh CLI (gh auth login), if installed

Flags:
`)
		flag.PrintDefaults()
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}
	if *version {
		printVersion()
		os.Exit(0)
	}
	if *user == "" || *org == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *month < 1 || *month > 12 {
		log.Fatal("month must be between 1 and 12")
	}
	if flag.NArg() > 0 {
		log.Fatal("provided too many arguments")
	}

	since, until := monthRange(*year, *month)

	return config{
		user:  *user,
		org:   *org,
		since: since,
		until: until,
	}
}

func printVersion() {
	fmt.Printf("footprint %s (commit %s, built %s)\n", version, commit, date)
}

func resolveToken() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}

	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		log.Fatal("no token: set GH_TOKEN or run `gh auth login`")
	}

	return strings.TrimSpace(string(out))
}

func monthRange(year, month int) (since, until time.Time) {
	since = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	until = since.AddDate(0, 1, 0).Add(-time.Nanosecond)
	return since, until
}

func shortSHA(sha string) string {
	if len(sha) < 7 {
		return sha
	}
	return sha[:7]
}

func getMergeCommitSHAs(
	ctx context.Context,
	client *github.Client,
	org,
	repo string,
	pr int,
) ([]string, error) {
	opt := &github.ListOptions{PerPage: 100}
	var shas []string

	for {
		commits, res, err := client.PullRequests.ListCommits(ctx, org, repo, pr, opt)
		if err != nil {
			return nil, err
		}
		for _, c := range commits {
			sha := shortSHA(c.GetSHA())
			shas = append(shas, sha)
		}
		if res.NextPage == 0 {
			break
		}
		opt.Page = res.NextPage
	}

	return shas, nil
}

func fetchCommitsByRepo(
	ctx context.Context,
	client *github.Client,
	config config,
) (map[string][]*github.CommitResult, error) {
	query := fmt.Sprintf(
		"author:%s org:%s committer-date:%s..%s",
		config.user,
		config.org,
		config.since.Format(time.DateOnly),
		config.until.Format(time.DateOnly),
	)

	opt := &github.SearchOptions{
		Sort:        "committer-date",
		Order:       "asc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	byRepo := make(map[string][]*github.CommitResult)
	for {
		result, res, err := client.Search.Commits(ctx, query, opt)
		if err != nil {
			return nil, err
		}

		for _, c := range result.Commits {
			repoName := c.GetRepository().GetName()
			byRepo[repoName] = append(byRepo[repoName], c)
		}

		if res.NextPage == 0 {
			break
		}
		opt.Page = res.NextPage
	}

	return byRepo, nil
}

func buildReports(
	ctx context.Context,
	client *github.Client,
	org string,
	byRepo map[string][]*github.CommitResult,
) []report {
	prRe := regexp.MustCompile(`\(#(\d+)\)$`)
	reports := make([]report, 0, len(byRepo))

	for _, repoName := range slices.Sorted(maps.Keys(byRepo)) {
		commits := byRepo[repoName]
		messages := make([]string, 0, len(commits))

		for _, c := range commits {
			summary := strings.SplitN(c.GetCommit().GetMessage(), "\n", 2)[0]
			sha := shortSHA(c.GetSHA())

			if m := prRe.FindStringSubmatch(summary); m != nil {
				prNum, _ := strconv.Atoi(m[1])
				clean := strings.TrimSpace(prRe.ReplaceAllString(summary, ""))
				shas, err := getMergeCommitSHAs(ctx, client, org, repoName, prNum)
				if err != nil {
					shas = []string{sha}
					log.Printf("#%d in %s: falling back to merge SHA", prNum, clean)
				}
				messages = append(messages, fmt.Sprintf("#%d: %s (%s)", prNum, clean, strings.Join(shas, ", ")))
			} else {
				messages = append(messages, fmt.Sprintf("%s (%s)", summary, sha))
			}
		}
		reports = append(reports, report{repoName, messages})
	}
	return reports
}

func printReport(report report) {
	fmt.Printf("Repository \"%s\" contributions:\n", report.name)
	for _, m := range report.messages {
		fmt.Println(m)
	}
	fmt.Println()
}
