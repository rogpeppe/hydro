install:
	(cd statik; ./gen.sh)
	go generate ./meterstore/internal/meterstorepb
	go install ./...
