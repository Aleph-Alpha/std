# Makefile at project root

.PHONY: docs test build clean

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
	@echo "# Go Packages Documentation" > docs/README.md
	@echo "" >> docs/README.md
	@echo "Generated on $$(date)" >> docs/README.md
	@echo "" >> docs/README.md
	@echo "## Packages" >> docs/README.md
	@for pkg in $$(find pkg -maxdepth 1 -mindepth 1 -type d | grep -v "examples"); do \
		pkgname=$$(basename $$pkg); \
		echo "- [$$pkgname](pkg/$$pkgname.md)" >> docs/README.md; \
	done
	@echo "Documentation generated in docs/ directory"

# Clean generated documentation
clean:
	@echo "Cleaning generated documentation..."
	@rm -rf docs/pkg/*.md 2>/dev/null || true
	@echo "Documentation cleaned."

# Build the project
build:
	go build ./...

# Run tests
test:
	go test ./...