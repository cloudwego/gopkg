# How to Contribute

## Your First Pull Request
We use GitHub for our codebase. You can start by reading [How To Pull Request](https://docs.github.com/en/github/collaborating-with-issues-and-pull-requests/about-pull-requests).

## Branch Organization
We use [git-flow](https://nvie.com/posts/a-successful-git-branching-model/) as our branch organization, also known as [FDD](https://en.wikipedia.org/wiki/Feature-driven_development).

## Bugs

### 1. How to Find Known Issues
We use [GitHub Issues](https://github.com/cloudwego/gopkg/issues) for public bugs. We keep a close eye on this and try to make it clear when an internal fix is in progress. Before filing a new issue, please check that your problem doesn’t already exist.

### 2. Reporting New Issues
Providing a minimal reproducing test case is the recommended way to report issues. It can be placed in:
- The issue itself
- [Go Playground](https://play.golang.org/)

### 3. Security Bugs
Please do not report security vulnerabilities through public issues. Contact us via [Support Email](mailto:conduct@cloudwego.io).

## How to Get in Touch
- [Email](mailto:conduct@cloudwego.io)

## Submit a Pull Request
Before submitting your Pull Request (PR), consider the following guidelines:
1. Search [GitHub](https://github.com/cloudwego/gopkg/pulls) for an open or closed PR related to your submission to avoid duplicating effort.
2. Make sure an issue describes the problem you’re fixing or documents the design for the feature you’d like to add. Discussing the design upfront helps ensure we’re ready to accept your work.
3. [Fork](https://docs.github.com/en/github/getting-started-with-github/fork-a-repo) the cloudwego/gopkg repo.
4. In your forked repository, make your changes in a new git branch:
    ```
    git checkout -b my-fix-branch main
    ```
5. Create your patch, including appropriate test cases.
6. Follow our [Style Guides](#code-style-guides).
7. Commit your changes using a descriptive commit message following [AngularJS Git Commit Message Conventions](https://docs.google.com/document/d/1QrDFcIiPjSLDn3EL15IJygNPiHORgU1_OOAqWjiDU5Y/edit).
   Adherence to these conventions is necessary because release notes are automatically generated from these messages.
8. Push your branch to GitHub:
    ```
    git push origin my-fix-branch
    ```
9. In GitHub, send a pull request to `cloudwego/gopkg:main`.

## Contribution Prerequisites
- Our development environment tracks [Go official releases](https://golang.org/project/).
- Run lint tools before submitting your PR: [gofmt](https://golang.org/pkg/cmd/gofmt/) and [golangci-lint](https://github.com/golangci/golangci-lint).
- Familiarity with [GitHub](https://github.com) and [GitHub Actions](https://github.com/features/actions) (our CI tool) is helpful.

## Dependencies

**We prefer to avoid introducing third-party dependencies.**

This repository is a foundational package used across the CloudWeGo ecosystem. Keeping the dependency tree lean reduces upgrade friction, supply chain risk, and binary bloat for all downstream users.

Before adding a new dependency, ask yourself:
- Can this be implemented with a reasonable amount of code using only the Go standard library?
- Is the dependency actively maintained and widely trusted?
- Does the benefit clearly outweigh the cost of a new transitive dependency?

If the answer to the first question is "yes", implement it directly. For test-only needs, prefer the `internal/assert` package already provided in this repo over pulling in external test frameworks.

## Code Style Guides
Also see [PingCAP General Advice](https://pingcap.github.io/style-guide/general.html).

Good resources:
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
