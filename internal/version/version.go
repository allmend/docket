// Package version carries the application version string.
package version

// Version is the released version of Docket, shown in the UI.
//
// Release builds override it via ldflags:
//
//	go build -ldflags "-X github.com/allmend/docket/internal/version.Version=1.2.3"
//
// The literal below is the fallback for plain `go build` / `go run`, so keep it
// in step with the newest CHANGELOG.md entry when tagging a release.
var Version = "0.12.0"
