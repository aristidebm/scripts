.PHONY: serve telnet netcat format

serve:
	@go run chat.go

telnet:
	@telnet -e 'x' localhost 8000

netcat:
	@netcat localhost 8000

format:
	@go fmt .
