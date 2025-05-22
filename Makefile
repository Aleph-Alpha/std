# Makefile at project root

.PHONY: docs test build clean test-with-containers

# Generate documentation using gomarkdoc
docs:
	@echo "Generating Markdown documentation..."
	@mkdir -p docs/pkg
	@rm -rf docs/pkg/*.md
	@for pkg in $$(find pkg -maxdepth 1 -mindepth 1 -type d | grep -v "examples"); do \
		pkgname=$$(basename $$pkg); \
		echo "Processing $$pkgname..."; \
		gomarkdoc ./$$pkg \
			--output "docs/pkg/$$pkgname.md" \
			--repository.url "https://gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages" \
			--format github; \
	done
	@echo "Updating root README.md with package links..."
	# Create a temp file with documentation section
	@echo "# Go Packages Documentation" > docs_section.tmp
	@echo "" >> docs_section.tmp
	@echo "Generated on $$(date)" >> docs_section.tmp
	@echo "" >> docs_section.tmp
	@echo "## Packages" >> docs_section.tmp
	@for pkg in $$(find pkg -maxdepth 1 -mindepth 1 -type d | grep -v "examples"); do \
		pkgname=$$(basename $$pkg); \
		echo "- [$$pkgname](docs/pkg/$$pkgname.md)" >> docs_section.tmp; \
	done
	
	# Replace existing documentation section or append to README.md
	@if grep -q "# Go Packages Documentation" README.md; then \
		awk 'BEGIN {p=1} /# Go Packages Documentation/{p=0} /^# [^G]/{p=1} p' README.md > README.tmp; \
		cat docs_section.tmp >> README.tmp; \
		mv README.tmp README.md; \
	else \
		cat docs_section.tmp >> README.md; \
	fi
	@rm docs_section.tmp
	@echo "Documentation generated in docs/pkg/ directory and listed in README.md"

# Clean generated documentation
clean:
	@echo "Cleaning generated documentation..."
	@rm -rf docs/pkg/*.md 2>/dev/null || true
	@rm -f docs_section.tmp README.tmp 2>/dev/null || true
	@echo "Documentation cleaned."

# Build the project
build:
	go build ./...

# Run tests
test:
	go test -v ./...

# Test with containers - auto-detect Colima and set appropriate variables
test-with-containers:
	@if [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		echo "Colima detected, running tests with Colima configuration..."; \
		DOCKER_HOST=unix://$$HOME/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test -v ./...; \
	else \
		echo "Colima not detected, running standard tests..."; \
		go test -v ./...; \
	fi
