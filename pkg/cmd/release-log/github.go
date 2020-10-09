package release_log

import (
	"context"
	"os"
	"time"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

func listRepositoryCommits(org, repo string, since, until time.Time) ([]*github.RepositoryCommit, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	commits, _, err := client.Repositories.ListCommits(ctx, org, repo, &github.CommitsListOptions{
		Since: since,
		Until: until,
	})
	return commits, err
}
