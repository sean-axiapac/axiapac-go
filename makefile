# Go parameters
GO       ?= go
GOOS     ?= linux
GOARCH   ?= arm64
BINARY   = bootstrap
LAMBDA   = axiapac-reply-email-handler
SRC      = ./lambdas/$(LAMBDA)
BUILD    = ./build/$(LAMBDA)
FUNCTION = axiapac-reply-email-handler
ZIP      = ./build/$(LAMBDA).zip

.DEFAULT_GOAL := no-task

.PHONY: oktedi no-task clean build zip upload deploy

oktedi:
	$(MAKE) -f ./oktedi/makefile.mk $(filter-out $@,$(MAKECMDGOALS))

no-task:
	@echo "❌ You must specify a task (e.g. make build, make zip, make deploy)"
	@exit 1


deploy: clean build zip upload

build:
	@echo "Building Lambda $(LAMBDA)..."
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o $(BUILD)/$(BINARY) $(SRC)

zip: build
	@echo "Packaging Lambda $(LAMBDA)..."
	cd $(BUILD) && zip -r9 ../$(LAMBDA).zip $(BINARY)

upload: zip
	@echo "Deploying Lambda to AWS: $(FUNCTION)"
	aws lambda update-function-code \
		--function-name $(FUNCTION) \
		--zip-file fileb://$(ZIP)

clean:
	@echo "Cleaning build artifacts..."
	rm -rf build