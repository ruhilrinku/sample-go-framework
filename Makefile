.PHONY: generate build test clean run db-update db-rollback db-status db-validate

generate:
	go run github.com/bufbuild/buf/cmd/buf@v1.50.0 generate
	rm -rf gen/pb/google

build:
	go build ./...

test:
	go test ./... -v -count=1

clean:
	rm -rf gen/pb/

run:
	go run ./cmd/server/

db-update:
	liquibase --defaults-file=liquibase.properties update

db-rollback:
	liquibase --defaults-file=liquibase.properties rollback-count 1

db-status:
	liquibase --defaults-file=liquibase.properties status

db-validate:
	liquibase --defaults-file=liquibase.properties validate
