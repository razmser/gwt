# Build the gwt binary
build:
    go build .

# Install gwt to ~/bin along with fish autocomplete
install: build
    @mkdir -p ~/bin
    @mkdir -p ~/.config/fish/completions
    @cp gwt ~/bin/
    @cp autocomplete.fish ~/.config/fish/completions/gwt.fish
    @echo "Installed gwt to ~/bin/gwt"
    @echo "Installed gwt completions to ~/.config/fish/completions/gwt.fish"

# Clean build artifacts
clean:
    rm -f gwt

# Run tests
test:
    @echo "Running tests..."
    go test -timeout 60s ./...

# Run linters
lint:
    @echo "Running linters..."
    golangci-lint run

# Run all checks
check: lint test

# Show available commands
help:
    @just --list
