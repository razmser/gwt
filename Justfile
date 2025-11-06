# Build the git-wt binary
build:
    go build .

# Run linters
lint:
    @echo "Running linters..."
    golangci-lint run

# Run tests
test:
    @echo "Running tests..."
    go test ./...

# Run all checks
check: lint test

# Clean build artifacts
clean:
    rm -f gwt

install: build
    @mkdir -p ~/bin
    @mkdir -p ~/.config/fish/completions
    @cp gwt ~/bin/
    @cp autocomplete.fish ~/.config/fish/completions/gwt.fish
    @echo "Installed gwt to ~/bin/gwt"
    @echo "Installed gwt completions to ~/.config/fish/completions/gwt.fish"

# Show available commands
help:
    @just --list
