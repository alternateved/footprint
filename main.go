package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	ghToken := os.Getenv("GH_TOKEN")
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

	repos, err := fetchRepositories(client, org)
	if err != nil {
		log.Fatal(err)
	}

	for _, repo := range repos {
		messages, err := getRepositoryReport(client, repo, user, org, since, until)
		if err != nil {
			log.Fatal(err)
		}
		printReport(repo.GetName(), messages)
	}
}

func fetchRepositories(client *github.Client, org string) ([]*github.Repository, error) {
	var repos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{
		Type:        "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		rep, res, err := client.Repositories.ListByOrg(context.Background(), org, opt)
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

func getMergeCommitSHAs(client *github.Client, org, repo string, pr int) []string {
	opt := &github.ListOptions{PerPage: 100}
	var shas []string

	for {
		commits, res, err := client.PullRequests.ListCommits(context.Background(), org, repo, pr, opt)
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

func getRepositoryReport(client *github.Client, repo *github.Repository, user, org string, since, until time.Time) ([]string, error) {
	repoName := repo.GetName()
	prRe := regexp.MustCompile(`\(#(\d+)\)`)
	opt := &github.CommitsListOptions{
		Author:      user,
		Since:       since,
		Until:       until,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var messages []string
	for {
		commits, res, err := client.Repositories.ListCommits(context.Background(), org, repoName, opt)
		if err != nil {
			return nil, err
		}

		for _, c := range commits {
			msg := c.GetCommit().GetMessage()
			summary := strings.SplitN(msg, "\n", 2)[0]

			if m := prRe.FindStringSubmatch(summary); m != nil {
				prNum, _ := strconv.Atoi(m[1])
				cleanSummary := strings.TrimSpace(prRe.ReplaceAllString(summary, ""))
				shas := getMergeCommitSHAs(client, org, repoName, prNum)
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

func printReport(repoName string, messages []string) {
	if len(messages) > 0 {
		fmt.Printf("Repository \"%s\" contributions:\n", repoName)
		for _, m := range messages {
			fmt.Println(m)
		}
		fmt.Println()
	}
}
