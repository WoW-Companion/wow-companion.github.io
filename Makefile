.PHONY: verify

# verify checks links, images, page basics, external URLs, identity leaks and
# HTML well-formedness. Standard library Go only.
verify:
	@test -z "$$(gofmt -l tools)" || { echo "gofmt needed:"; gofmt -l tools; exit 1; }
	go vet ./tools/...
	go run ./tools/verify .
