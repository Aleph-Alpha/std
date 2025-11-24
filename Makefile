# Makefile at project root

# Allow user to override the Go binary path
GO ?= go

.PHONY: docs test build clean test-with-containers lint fmt install-tools

# Generate documentation using gomarkdoc
docs:
	@echo "Generating Markdown documentation..."
	@echo "Checking for gomarkdoc..."
	@export PATH="$$($(GO) env GOBIN):$$($(GO) env GOPATH)/bin:$$PATH"; \
	which gomarkdoc >/dev/null 2>&1 || { \
		echo "gomarkdoc not found, installing..."; \
		$(GO) install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest; \
		echo "gomarkdoc installed successfully"; \
	}; \
	mkdir -p docs/v1; \
	rm -rf docs/v1/*.md; \
	for pkg in $$(find v1 -maxdepth 1 -mindepth 1 -type d | grep -v "examples"); do \
		pkgname=$$(basename $$pkg); \
		echo "Processing $$pkgname..."; \
		gomarkdoc ./$$pkg \
			--output "docs/v1/$$pkgname.md" \
			--repository.url "https://github.com/Aleph-Alpha/std" \
			--format github; \
	done; \
	echo "Updating root README.md with package links..."; \
	echo "# Go Packages Documentation" > docs_section.tmp; \
	echo "" >> docs_section.tmp; \
	echo "Generated on $$(date)" >> docs_section.tmp; \
	echo "" >> docs_section.tmp; \
	echo "## Packages" >> docs_section.tmp; \
	for pkg in $$(find v1 -maxdepth 1 -mindepth 1 -type d | grep -v "examples"); do \
		pkgname=$$(basename $$pkg); \
		echo "- [$$pkgname](docs/v1/$$pkgname.md)" >> docs_section.tmp; \
	done; \
	if grep -q "# Go Packages Documentation" README.md; then \
		awk 'BEGIN {p=1} /# Go Packages Documentation/{p=0} /^# [^G]/{p=1} p' README.md > README.tmp; \
		cat docs_section.tmp >> README.tmp; \
		mv README.tmp README.md; \
	else \
		cat docs_section.tmp >> README.md; \
	fi; \
	rm docs_section.tmp; \
	echo "Documentation generated in docs/v1/ directory and listed in README.md"

# Clean generated documentation and test artifacts
clean:
	@echo "Cleaning generated documentation and test artifacts..."
	@rm -rf docs/v1/*.md 2>/dev/null || true
	@rm -f docs_section.tmp README.tmp test_output.log 2>/dev/null || true
	@echo "Documentation and test artifacts cleaned."

# Build the project
build:
	$(GO) build ./...

# Function to generate test summary
define test_summary
	@echo ""
	@echo "===================="
	@echo "TEST SUMMARY"
	@echo "===================="
	@total_tests=$$(grep -E "^=== RUN" test_output.log | wc -l | tr -d ' '); \
	passed_tests=$$(grep -E "^    --- PASS:|^--- PASS:|^        --- PASS:" test_output.log | wc -l | tr -d ' '); \
	failed_tests=$$(grep -E "^    --- FAIL:|^--- FAIL:|^        --- FAIL:" test_output.log | wc -l | tr -d ' '); \
	skipped_tests=$$(grep -E "^    --- SKIP:|^--- SKIP:|^        --- SKIP:" test_output.log | wc -l | tr -d ' '); \
	cont_tests=$$(grep -E "^    --- CONT:|^--- CONT:|^        --- CONT:" test_output.log | wc -l | tr -d ' '); \
	echo "Total tests: $$total_tests"; \
	echo "Passed: $$passed_tests"; \
	echo "Failed: $$failed_tests"; \
	echo "Skipped: $$skipped_tests"; \
	if [ $$cont_tests -gt 0 ]; then echo "Continued: $$cont_tests"; fi; \
	accounted_tests=$$(($$passed_tests + $$failed_tests + $$skipped_tests)); \
	if [ $$accounted_tests -ne $$total_tests ]; then \
		unaccounted=$$(($$total_tests - $$accounted_tests)); \
		echo "Unaccounted: $$unaccounted"; \
		echo "Breakdown by indentation level:"; \
		top_level=$$(grep -E "^--- (PASS|FAIL|SKIP):" test_output.log | wc -l | tr -d ' '); \
		sub_level=$$(grep -E "^    --- (PASS|FAIL|SKIP):" test_output.log | wc -l | tr -d ' '); \
		deep_level=$$(grep -E "^        --- (PASS|FAIL|SKIP):" test_output.log | wc -l | tr -d ' '); \
		echo "  Top-level results: $$top_level"; \
		echo "  Sub-test results: $$sub_level"; \
		echo "  Deep sub-test results: $$deep_level"; \
		echo "  Total result lines: $$(($$top_level + $$sub_level + $$deep_level))"; \
	fi; \
	if [ $$failed_tests -gt 0 ]; then \
		echo ""; \
		echo "==================== FAILED TEST DETAILS ===================="; \
		grep -A 10 -B 2 -E "^    --- FAIL:|^--- FAIL:" test_output.log || true; \
		echo ""; \
		echo "==================== FULL LOGS FOR FAILED TESTS ===================="; \
		awk '/^=== RUN.*/{test=$$3} /^    --- FAIL:|^--- FAIL:.*/{print "FAILED TEST: " test; in_fail=1; next} /^    --- PASS:|^--- PASS:.*/{in_fail=0} /^=== RUN.*/{if(in_fail) print ""} in_fail && !/^(===|    ---|---)/  {print}' test_output.log; \
	fi
	@rm -f test_output.log
endef

# Run tests with detailed summary
test:
	@echo "Running tests..."
	@echo "===================="
	@$(GO) test -v ./... 2>&1 | tee test_output.log
	$(call test_summary)

# Test with containers - auto-detect Colima and set appropriate variables with detailed summary
test-with-containers:
	@echo "Running tests with containers..."
	@echo "===================="
	@if [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		echo "Colima detected, running tests with Colima configuration..."; \
		DOCKER_HOST=unix://$$HOME/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true $(GO) test -v ./... 2>&1 | tee test_output.log; \
	else \
		echo "Colima not detected, running standard tests..."; \
		$(GO) test -v ./... 2>&1 | tee test_output.log; \
	fi
	$(call test_summary)

# Run linter
lint:
	@echo "Running linter..."
	@# Ensure we use the GOROOT from the selected GO binary and add its bin to PATH
	GOROOT=$$($(GO) env GOROOT) PATH="$$($(GO) env GOPATH)/bin:$$($(GO) env GOROOT)/bin:$$PATH" golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	goimports -w .

# Install development tools
install-tools:
	@echo "Installing tools..."
	$(GO) install github.com/evilmartians/lefthook@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	@echo "Installing golangci-lint from source..."
	# Using direct proxy as fallback for transient git errors if any
	GOPROXY=https://proxy.golang.org,direct $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
