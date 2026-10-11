# Bitcoin Mining Pool Demo (Go)

This repository contains a local Go-based Bitcoin mining pool demo for testing the pool architecture.

What it does:
- starts a TCP mining-pool server on `127.0.0.1:3333`
- accepts miner connections
- sends mining jobs to clients
- validates submitted shares
- shows how a pool and miner communicate

This is a demo implementation, not a production Bitcoin mining pool or a live network deployment.

## Run the demo

1. Start the pool:
   ```bash
   go run ./cmd/pool
   ```

2. Start a demo miner in another terminal:
   ```bash
   go run ./examples/miner
   ```

3. The pool will print logs and the miner will connect and submit a share.

## Local address

- Pool server: `127.0.0.1:3333`
- Demo client: connects to `127.0.0.1:3333`

## Important note

This is a learning/demo project only. A real production Bitcoin mining pool needs a full Bitcoin Core node, valid block templates, worker database, payout system, and secure deployment.
