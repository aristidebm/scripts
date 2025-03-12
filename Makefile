.PHONY: serve telnet netcat

serve:
	@go run chat.go

telnet:
	@telnet -e 'x' localhost 8000

netcat:
	@netcat localhost 8000