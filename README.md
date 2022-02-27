<h1 align="center">DStat 📊</h1>

Get statistics about your Discord bot!

### Usage:
**Running with Docker (recommended)**
`docker run -t benricheson101/dstat -token <your-token>`

**Build from source**
```bash
$ git clone git@github.com:benricheson101/dstat.git
$ cd dstat
$ go run main.go -token <your-token>
```

### Command-Line Flags
```
Usage of dstat:
  -json
    	output JSON instead of a formatted list. useful for programmatic usage
  -nolive
    	disables live output
  -timeout string
    	the time to wait for guilds to become available (default "20s")
  -token string
    	the discord token to connect with (default: "$DISCORD_TOKEN" environment variable)
```
