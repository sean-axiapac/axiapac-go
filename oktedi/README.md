### build
```
GOOS=linux GOARCH=amd64 go build -C ./oktedi/web -o ../dist/server
aws s3 sync ./oktedi/dist/ s3://axiapac-development/oktedi/ --delete
```

### deploy
```
sudo aws s3 sync s3://axiapac-development/oktedi/ /apps/oktedi/ --exact-timestamps --delete
sudo chmod 755 server
```