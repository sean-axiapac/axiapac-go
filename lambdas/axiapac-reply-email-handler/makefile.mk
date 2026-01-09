FUNCTION = axiapac-reply-email-handler
DIR      = ./lambdas/$(FUNCTION)
DIST     = $(DIR)/dist
OUTPUT    = $(DIST)/bootstrap
ZIP      = $(OUTPUT).zip

.DEFAULT_GOAL := no-task

.PHONY: no-task clean build zip upload deploy

no-task:
	@echo "❌ You must specify a task (e.g. make build, make zip, make deploy)"
	@exit 1

deploy: clean build zip upload

build:
	@echo "Building Lambda $(FUNCTION)..."
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=arm64 go build -o $(OUTPUT) $(DIR)

zip: build
	@echo "Packaging Lambda $(FUNCTION)..."
	zip -j $(ZIP) $(OUTPUT)

upload: zip
	@echo "Deploying Lambda to AWS: $(FUNCTION)"
	aws lambda update-function-code \
		--function-name $(FUNCTION) \
		--zip-file fileb://$(ZIP)

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(DIST)