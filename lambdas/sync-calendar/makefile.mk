.PHONY: no-task clean build zip upload deploy run

no-task:
	@echo "❌ You must specify a task (e.g. make build, make zip, make deploy, make run)"
	@exit 1

run:
	go run ./lambdas/sync-calendar

build:
	GOOS=linux GOARCH=arm64 go build -o ./lambdas/sync-calendar/bootstrap ./lambdas/sync-calendar

zip: build
	@echo "Packaging Lambda sync-calendar..."
	cd ./lambdas/sync-calendar && zip -r9 ./bootstrap.zip bootstrap

upload: zip
	@echo "Deploying Lambda to AWS: sync-calendar"
	aws lambda update-function-code \
		--function-name sync-calendar \
		--zip-file fileb://./lambdas/sync-calendar/bootstrap.zip

deploy: clean build zip upload

clean:
	@echo "Cleaning build artifacts..."
	rm -rf build