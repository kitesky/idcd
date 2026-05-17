# Pebble integration tests

`TestLetsEncrypt_Integration_Pebble` exercises the full ACME flow (newOrder →
DNS-01 → finalize → revoke) against a local [Pebble](https://github.com/letsencrypt/pebble)
server + `pebble-challtestsrv` mock DNS. The harness lives in
`lib/cert/ca/internal/pebble`.

When the host environment can't run the integration the harness returns
`pebble.ErrSkip` and the test calls `t.Skip` — `go test ./...` stays green
out of the box.

## How to run

### Linux dev box (docker daemon running)

```bash
# Pull the images once
docker pull letsencrypt/pebble:latest
docker pull letsencrypt/pebble-challtestsrv:latest

# Run the test; harness brings up + tears down its own containers
go test ./lib/cert/ca/letsencrypt/... -run TestLetsEncrypt_Integration_Pebble -v
```

Total wall-clock ~15-25s including `docker pull` cache hits and Pebble's
account-registration round trip.

### CI / Mac / Windows (external Pebble via docker-compose)

```bash
docker compose -f lib/cert/ca/internal/pebble/docker-compose.yml up -d

# Tell the harness to use the already-running pair
export PEBBLE_DIRECTORY_URL=https://127.0.0.1:14000/dir
export PEBBLE_CHALLTESTSRV_URL=http://127.0.0.1:8055
export PEBBLE_DNS_SERVER=127.0.0.1:8053

# Trust Pebble's self-signed CA root
docker compose -f lib/cert/ca/internal/pebble/docker-compose.yml \
    exec -T pebble cat /test/certs/pebble.minica.pem > /tmp/pebble.minica.pem
export LEGO_CA_CERTIFICATES=/tmp/pebble.minica.pem

go test ./lib/cert/ca/letsencrypt/... -run TestLetsEncrypt_Integration_Pebble -v

docker compose -f lib/cert/ca/internal/pebble/docker-compose.yml down
```

## Environment requirements

| Requirement | Notes |
|---|---|
| Linux | docker-mode (no env vars set) requires `--network host`, which only works on Linux. Mac / Windows users use env-var mode. |
| `docker` binary in `PATH` | Required for docker mode. |
| Reachable docker daemon | `docker info` must succeed; otherwise the test skips. |
| Ports `14000`, `15000`, `8053`, `8055` free | Docker-compose maps these on the host. |
| Outbound network for image pull (first run only) | Or pre-pull `letsencrypt/pebble:latest` and `letsencrypt/pebble-challtestsrv:latest`. |

## Common failures

| Symptom | Cause / fix |
|---|---|
| `SKIP pebble: docker daemon not reachable` | The daemon isn't running. Start Docker Desktop / `sudo systemctl start docker` / set `DOCKER_HOST`. |
| `SKIP pebble: docker mode only supported on linux` | You're on Mac or Windows. Bring up the docker-compose fixture and export `PEBBLE_DIRECTORY_URL` as shown above. |
| `RequestCertificate: ca: network ...: x509: certificate signed by unknown authority` | `LEGO_CA_CERTIFICATES` not pointing at Pebble's root. Re-export it from the running container (see compose snippet above). |
| `RequestCertificate: ca: authorization invalid: dns ... no records found` | challtestsrv's DNS port `8053` isn't reachable from Pebble, or Pebble was started without `-dnsserver host:8053`. The harness wires this correctly; only relevant when bringing your own Pebble. |
| Test hangs ~5+ minutes | Pebble's deliberate VA reorder delay. The harness sets `PEBBLE_VA_NOSLEEP=1`; if you brought your own Pebble, set it there too. |
