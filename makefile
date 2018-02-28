pi:
	env GOOS=linux GOARCH=arm go build -o goazurekeyvault main.go
windows:
	env GOOS=windows GOARCH=amd64 go build -o goazurekeyvault.exe main.go
