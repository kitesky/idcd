# idcd-go

Official Go SDK for the [idcd](https://idcd.com) API.

## Installation

```
go get github.com/kite365/idcd-go
```

## Quick Start

```go
import idcd "github.com/kite365/idcd-go"

client := idcd.New("sk_live_your_api_key")

// Probe a URL
result, err := client.ProbeHTTP(ctx, idcd.ProbeHTTPRequest{URL: "https://example.com"})

// List monitors
monitors, err := client.ListMonitors(ctx)

// Query IP info
info, err := client.GetIPInfo(ctx, "1.2.3.4")
```

## Configuration

```go
client := idcd.New("sk_live_your_api_key",
    idcd.WithBaseURL("https://api.idcd.com"),
    idcd.WithTimeout(15 * time.Second),
)
```

## License

MIT
