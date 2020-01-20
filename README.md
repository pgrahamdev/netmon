= Netmon - Simple Go wrapper for SpeedTest.net Python with Web interface

This is an experiment on how to write a Go program that interacts with a Web interface through WebSockets.  There is an accompanying repository with a React web interface intended to work with the Go program.

To use the program, you will need the following:

1. Clone and build this repository with `go` (`go build .` from the cloned `netmon` directory).
2. Clone the [React Web GUI](https://github.com/pgrahamdev/netmon-react) and compile (See the instructions in [README.md](https://github.com/pgrahamdev/netmon-react/blob/master/README.md)).
3. Place the contents of the React Web GUI's `build` directory into the `www` subdirectory relative to where the `netmon` program is run.
4. Include the `speedtest-cli` program from [Github](https://github.com/sivel/speedtest-cli) in your search path.

At this point, the `netmon` program can be run.

The usage for the program is as follows:

    Usage of ./netmon:
    -addr string
            http service address (default ":8080")
    -period int
            The period between calls to speedtest-cli (default 60)
    -server int
            The server ID to use for speedtest-cli (default -1)
