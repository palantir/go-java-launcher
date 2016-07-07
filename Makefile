build:
	@scripts/build.sh

test:
	@scripts/verify.sh

verify:
	@scripts/verify.sh -c

lint:
	@scripts/lint.sh

dist:
	@scripts/dist.sh

publish:
	@scripts/publish.sh

.PHONY: build test verify lint dist publish
