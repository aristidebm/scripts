## Channel

This project is an attempt to understand how chats maintains connection between different participants

## Fixes

- [ ] There is a problem now, below is a scenario to reproduce the problem
  1. connect to the server with the telnet client.
  2. connect to the server with netcat client.
  3. Begin seding message from the telnet.
  4. The netcat client receive messages posted by the telnet client.
  5. Disconnect the telnet client by hitting Ctrl+C or `x` (you will be put inside a prompt) follow `quit`
  it the prompt.
  6. **Notice that a bunch of messages are offloaded to netcat client**, find the bug and fix it.