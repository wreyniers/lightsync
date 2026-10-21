# go-vee patch: UDP shutdown error

This is a local patch of [github.com/swrm-io/go-vee](https://github.com/swrm-io/go-vee).

**Issue**: During app shutdown, the Govee controller closes its UDP socket (port 4002) while the listener goroutine may still be blocked on `ReadFromUDP`. That returns "use of closed network connection", which the upstream library logs as ERROR even though it's expected during teardown.

**Fix**: Treat `net.ErrClosed` and "use of closed network connection" as normal shutdown — return from the goroutine without logging. See `controller.go` lines 78–82.

**Remove when**: Upstream merges equivalent handling. Consider opening a PR to swrm-io/go-vee.
