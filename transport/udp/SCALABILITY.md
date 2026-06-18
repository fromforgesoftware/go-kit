# UDP transport — scalability roadmap

Status of `transport/udp` and the prioritized work needed to reach high
concurrent-session scale (target reference: a single game realm, ~3k+ concurrent
players, ~100k inbound packets/sec at 30 Hz movement).

This document is a TODO backlog, not a description of shipped behaviour. Items
marked **DONE** are in `main`; the rest are future work, roughly in priority
order. Re-validate the ceiling with a load test before investing — several of
these are real re-architectures and should not be done blind.

## Current state (as of the reliable-UDP dedup commit)

- **Race-free.** `go test -race ./transport/udp/` is clean. One read goroutine
  owns inbound decode + session creation (no concurrent session-map writes);
  per-session reliability state is under `session.mu`; `sessions` is a
  `sync.Map`; `net.UDPConn` writes are safe for concurrent use.
- **Correctness DONE.** `ProcessPacket` de-dups reliable packets by seq
  (1024-slot sliding window): a retransmit is re-ACKed but delivered to the
  handler at most once (previously a high-RTT client could make an action fire
  twice).
- **Hot-path allocs DONE (partial).** `readLoop` decodes straight from the read
  buffer (removed a redundant full-datagram `make+copy`; `Unmarshal` still
  copies the payload). `writeTo` no longer logs per packet.
- **`Server.LocalAddr()` DONE.** Reports the bound address (needed for `:0`).

## Priority 1 — inbound throughput: batch syscalls + multiple readers

The single `readLoop` does one `ReadFromUDP` per datagram. At ~100k pkt/s the
per-syscall overhead dominates a single core, and it cannot use more than one.

- [ ] **`recvmmsg` batch reads** via `golang.org/x/net/ipv4` `PacketConn.ReadBatch`
      — read up to ~64 datagrams per syscall. ~10–60× fewer syscalls on Linux.
      (No-op fallback on platforms without `recvmmsg`.)
- [ ] **`SO_REUSEPORT`**: bind N sockets to the same port, run N read goroutines;
      the kernel load-balances by flow (4-tuple) so a given client always lands
      on the same socket/goroutine — preserves per-session ordering without
      cross-goroutine coordination. Scales reads across cores.
- [ ] **`sendmmsg` batch writes** for the broadcast/fan-out path (one syscall for
      many datagrams to many peers).
- [ ] Benchmark: extend `BenchmarkServerSessionEchoParallel` into a sustained
      N-session throughput harness and record pkt/s + p99 before/after.

## Priority 2 — per-packet tracing cost

`dispatchOne` starts a tracer span for **every** inbound packet. At movement-
packet rates this is significant even when not exported.

- [ ] Gate per-packet spans behind a sampler or a `WithPacketTracing(false)`
      server option; default off (or head-sampled) on the game hot path.
- [ ] Document that latency-sensitive servers should inject a no-op / low-sample
      tracer.

## Priority 3 — allocation pressure

- [ ] **Inbound payload pool.** `Unmarshal` allocates a fresh payload slice per
      packet; it escapes into the per-session `inbound` channel and is freed
      after the handler runs. A `sync.Pool` with release-after-dispatch removes
      this steady-state alloc (needs a "done" signal from the dispatch worker).
- [ ] **Outbound `Marshal` pool.** `Packet.Marshal` allocates per send; pool the
      write buffer (released after `WriteToUDP` returns).

## Priority 4 — reliability protocol robustness

- [ ] **Adaptive RTO.** `ResendTimeout` is a fixed 300 ms; a client with RTT >
      300 ms self-inflicts spurious retransmits (now harmless thanks to dedup,
      but wasteful). Track per-session smoothed RTT + variance (Jacobson/Karels)
      and derive the timer.
- [ ] **Congestion control / pacing.** No window or pacing today; a lossy link
      just retries on a fixed timer. Consider a simple windowed scheme before
      large reliable payloads (chat history, inventory) ride this channel.
- [ ] **Multiple reliable channels (ENet-style).** Reliable delivery is currently
      a single arrival-ordered stream; an unrelated lost reliable (e.g. chat)
      can delay a later reliable (e.g. an ability confirm). Per-category channels
      keep independent reliables independent.
- [ ] **Give-up signal.** After `MaxRetries` a reliable packet is silently
      dropped (logged). Surface a callback/close so the app can resync or
      disconnect rather than diverge.

## Priority 5 — periodic O(n) scans

- [ ] `resendLoop` and `reapLoop` `Range` every session each tick (100 ms /
      reaper interval), each taking `session.mu`. Fine at thousands of sessions;
      if it shows up under load, replace the resend scan with a global timer
      wheel keyed by next-resend time.

## Notes / invariants to preserve

- The single-reader model is *why* session creation needs no lock. Any move to
  multiple readers (`SO_REUSEPORT`) must keep a given client pinned to one
  reader (kernel flow hashing does this) or reintroduce a creation lock.
- The dedup window (`seqRecvBufSize`) must stay comfortably larger than the max
  in-flight reliables a sender produces before `MaxRetries` gives up.
