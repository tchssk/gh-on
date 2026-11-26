package github

// MockClient is a test double for GitHub API operations.
// Each field is an optional function that, when set, handles the corresponding
// ClientOps method call. When nil, a reasonable default is returned.
type MockClient struct {
	NotificationsFn   func(string) ([]Notification, string, bool, error)
	SearchIssuesFn    func(string) ([]Issue, error)
	PullRequestHeadFn func(string, int) (string, error)
}

// Compile-time check that MockClient satisfies ClientOps.
var _ ClientOps = (*MockClient)(nil)

func (m *MockClient) Notifications(etag string) ([]Notification, string, bool, error) {
	if m.NotificationsFn != nil {
		return m.NotificationsFn(etag)
	}
	return nil, etag, true, nil
}

func (m *MockClient) SearchIssues(query string) ([]Issue, error) {
	if m.SearchIssuesFn != nil {
		return m.SearchIssuesFn(query)
	}
	return nil, nil
}

func (m *MockClient) PullRequestHead(repo string, number int) (string, error) {
	if m.PullRequestHeadFn != nil {
		return m.PullRequestHeadFn(repo, number)
	}
	return "", nil
}
