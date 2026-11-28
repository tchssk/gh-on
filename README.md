# gh-on

A [GitHub CLI](https://cli.github.com/) extension that runs a command when a GitHub notification matches a rule.

It runs on macOS, Linux and FreeBSD. Windows is not supported.

## Installation

```sh
gh extension install tchssk/gh-on
```

## Usage

Write rules to `~/.config/gh/gh-on/rules.yaml`. See [examples/rules.yaml](examples/rules.yaml). For the values of `reason`, see [Notification reasons](https://docs.github.com/en/rest/activity/notifications#about-notification-reasons). gh-on only reads the notifications you are participating in, so reasons such as `subscribed` never match.

```sh
# Show what would run
gh on run --dry-run --all

# Check notifications once and run the commands of matching rules
gh on run
```

To run it every minute, use launchd or cron. See [examples](examples).

When a command exits 0, gh-on records its targets as handled and does not run them again. When it fails, all of its targets are passed again on the next matching notification. Make the command exit non-zero on failure, and skip the targets it has already handled.

The records are kept per rule name, so renaming a rule runs its targets again.

## Security

The commands may read pull requests and issues written by other people. If a command runs an AI agent, that content can carry a prompt injection. Restrict rules to owners or repositories you trust, and do not give the commands more permissions than they need.

gh-on itself does not write to GitHub.
