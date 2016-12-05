# dpkg-buildpackage calls make, so <all> should be empty.
all:
	@echo "All does nothing"

help:
	@echo "all		- Build it all (called from dpkg-buildpackage)"
	@echo "pkg		- deps+pkg_only"
	@echo "pkg_only		- Only builds Debian package"
	@echo "deps		- Updates external dependencies"
	@echo "check		- runs: go vet/fmt/test, also on Pitchfork"
	@echo "tests		- Runs all Golang based tests"
	@echo "vtests		- Runs all Golang based tests (verbose)"

tests:
	@echo "Running Pitchfork tests..."
	@go test ./...

vtests:
	@echo "Running Pitchfork tests (verbose)..."
	@go test -v ./...

pkg: deps pkg_only

pkg_only:
	@echo "- Building Pitchfork Package..."
	@export GOPATH=${PWD}/ext/_gopath
	@dpkg-buildpackage -d -uc -us -S

deps:
	@echo "- Fetching Pitchfork Dependencies..."
	@rm -rf ./EpicEditor
	@echo "- Fetching EpicEditor..."
	@git clone https://github.com/OscarGodson/EpicEditor.git
	@rm -f share/webroot/js/epiceditor.min.js
	@ln -s ../../../EpicEditor/epiceditor/js/epiceditor.min.js share/webroot/js/
	@rm -f share/webroot/css/epiceditor/themes
	@mkdir -p share/webroot/css/epiceditor/
	@ln -s ../../../../EpicEditor/epiceditor/themes share/webroot/css/epiceditor/
	@echo "Fetching Pitchfork Dependencies... done"

check: deps
	@echo "- Running 'go vet'"
	@go vet ./...
	@echo "- Running 'go fmt'"
	@go fmt ./...
	@echo "Running 'go test' (verbose)"
	@go test -v ./...

.PHONY: all help tests vtests pkg pkg_only deps check
