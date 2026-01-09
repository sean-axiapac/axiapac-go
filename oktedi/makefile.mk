clean:
	@echo "Cleaning..."
	rm -rf ./oktedi/dist

build-server:
	GOOS=linux GOARCH=amd64 go build -C ./oktedi/web -o ../dist/server

build-client:
	@echo "Building client..."
	@cd ../axiapac-os && pnpm run build

build: clean build-server build-client 
	@echo "Build complete!"

upload:
	@echo "Uploading ./oktedi/dist/ to s3://axiapac-development/oktedi/..."
	aws s3 sync ./oktedi/dist/ s3://axiapac-development/oktedi/ --delete
	@echo "Upload complete!"

deploy: clean build upload