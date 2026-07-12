package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/joho/godotenv"
)

type report struct {
	name     string
	messages []string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	ghToken := os.Getenv("GH_TOKEN")

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	client, err := github.NewClient(github.WithAuthToken(ghToken))
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) < 5 {
		fmt.Println("Usage: footprint <USER> <ORG> <YEAR> <MONTH>")
	}

	user, org := os.Args[1], os.Args[2]

	year, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatal(err)
	}

	month, err := strconv.Atoi(os.Args[4])
	if err != nil {
		log.Fatal(err)
	}

	since := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	until := since.AddDate(0, 1, 0).Add(-time.Nanosecond)

	fmt.Printf("Report for user '%s' in organization '%s' (%s - %s)\n\n",
		user,
		org,
		since.Format(time.DateOnly),
		until.Format(time.DateOnly),
	)

	repos, err := fetchRepositories(ctx, client, org)
	if err != nil {
		log.Fatal(err)
	}

	reports := fetchReports(ctx, client, repos, user, org, since, until)
	for _, report := range reports {
		printReport(report)
	}
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
) []string {
	opt := &github.ListOptions{PerPage: 100}
	var shas []string

	for {
		commits, res, err := client.PullRequests.ListCommits(ctx, org, repo, pr, opt)
		if err != nil {
			break
		}
		for _, c := range commits {
			shas = append(shas, c.GetSHA()[:7])
		}
		if res.NextPage == 0 {
			break
		}
		opt.Page = res.NextPage
	}

	return shas
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
				shas := getMergeCommitSHAs(ctx, client, org, repoName, prNum)
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
	prRe := regexp.MustCompile(`\(#(\d+)\)`)
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
