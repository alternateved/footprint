package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/go-github/v89/github"
)

type report struct {
	name     string
	messages []string
}

func main() {
	user, org, year, month := initializeFlags()

	ghToken := resolveToken()
	client, err := github.NewClient(github.WithAuthToken(ghToken))
	if err != nil {
		log.Fatal(err)
	}

	since, until := monthRange(year, month)

	fmt.Printf("Report for user '%s' in organization '%s' (%s - %s)\n\n",
		user,
		org,
		since.Format(time.DateOnly),
		until.Format(time.DateOnly),
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	repos, err := fetchRepositories(ctx, client, org)
	if err != nil {
		log.Fatal(err)
	}

	reports := fetchReports(ctx, client, repos, user, org, since, until)
	for _, report := range reports {
		printReport(report)
	}
}

func initializeFlags() (string, string, int, int) {
	user := flag.String("u", "", "GitHub username")
	org := flag.String("o", "", "GitHub organization")
	year := flag.Int("y", time.Now().Year(), "year")
	month := flag.Int("m", int(time.Now().Month()), "month")

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

	if *user == "" || *org == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *month < 1 || *month > 12 {
		log.Fatal("month must be between 1 and 12")
	}

	return *user, *org, *year, *month
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

func fetchRepositories(
	ctx context.Context,
	client *github.Client,
	org string,
) ([]*github.Repository, error) {
	var repos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{
		Type:        "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		rep, res, err := client.Repositories.ListByOrg(ctx, org, opt)
		if err != nil {
			return nil, err
		}
		repos = append(repos, rep...)
		if res.NextPage == 0 {
			break
		}
		opt.Page = res.NextPage
	}

	return repos, nil
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
			shas = append(shas, c.GetSHA()[:7])
		}
		if res.NextPage == 0 {
			break
		}
		opt.Page = res.NextPage
	}

	return shas, nil
}

func getRepositoryReport(
	ctx context.Context,
	client *github.Client,
	user,
	org,
	repoName string,
	since,
	until time.Time,
	prRe *regexp.Regexp,
) ([]string, error) {
	opt := &github.CommitsListOptions{
		Author:      user,
		Since:       since,
		Until:       until,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var messages []string
	for {
		commits, res, err := client.Repositories.ListCommits(ctx, org, repoName, opt)
		if err != nil {
			return nil, err
		}

		for _, c := range commits {
			msg := c.GetCommit().GetMessage()
			summary := strings.SplitN(msg, "\n", 2)[0]

			if m := prRe.FindStringSubmatch(summary); m != nil {
				prNum, _ := strconv.Atoi(m[1])
				cleanSummary := strings.TrimSpace(prRe.ReplaceAllString(summary, ""))
				shas, err := getMergeCommitSHAs(ctx, client, org, repoName, prNum)
				if err != nil {
					return nil, err
				}
				messages = append(messages, fmt.Sprintf("#%d: %s (%s)", prNum, cleanSummary, strings.Join(shas, ", ")))
			} else {
				messages = append(messages, fmt.Sprintf("%s (%s)", summary, c.GetSHA()[:7]))
			}
		}
		if res.NextPage == 0 {
			break
		}
		opt.Page = res.NextPage
	}

	return messages, nil
}

func fetchReports(
	ctx context.Context,
	client *github.Client,
	repos []*github.Repository,
	user,
	org string,
	since,
	until time.Time,
) []report {
	var wg sync.WaitGroup
	prRe := regexp.MustCompile(`\(#(\d+)\)$`)
	reports := make([]report, len(repos))
	sem := make(chan struct{}, 5)

	for i, repo := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			repoName := repo.GetName()
			messages, err := getRepositoryReport(ctx, client, user, org, repoName, since, until, prRe)
			if err != nil {
				log.Printf("Failure for \"%s\" repository while fetching report: %v\n", repoName, err)
				return
			}
			reports[i] = report{repoName, messages}
		}()
	}
	wg.Wait()

	return reports
}

func printReport(report report) {
	if len(report.messages) > 0 {
		fmt.Printf("Repository \"%s\" contributions:\n", report.name)
		for _, m := range report.messages {
			fmt.Println(m)
		}
		fmt.Println()
	}
}
