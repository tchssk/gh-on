package github

// ClientOps defines the GitHub API operations gh-on uses.
// The concrete Client type satisfies this interface.
// Tests can substitute a MockClient.
type ClientOps interface {
	// Notifications returns notModified when etag is still current.
	Notifications(etag string) (ns []Notification, newETag string, notModified bool, err error)
	SearchIssues(query string) ([]Issue, error)
	PullRequestHead(repo string, number int) (string, error)
}

// Compile-time check that Client satisfies ClientOps.
var _ ClientOps = (*Client)(nil)
