# next steps

1. make a "router" on top of the ipc handler. I think what you want to do is to be able to basically break the handler into multiple handlers for different types of messages. Perhaps even using generics or something so you can specify the type of message that you expect to receive for that type.
