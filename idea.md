# concept:

inter-process communication between a process and it's subprocess, using the subprocess' stdio streams

goals:
- use a standard format like JSON for data exchange
- make a simple request/response protocol, ie the api surface should be basically `call(any) -> (any, error)` (available in both directions)
- implement a simple client/server library
- make a proof of concept in examples/

language:
- go
