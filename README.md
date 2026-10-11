# Bitcoin Mining Pool Starter

This repository is a lightweight Bitcoin mining pool prototype for learning and experimentation. It simulates a simplified mining-pool flow: worker registration, job assignment, difficulty checking, share validation, and pool statistics.

Important notes:
- This is not a production-ready Bitcoin node or mining pool.
- It is meant to demonstrate mining-pool concepts and architecture.
- Real Bitcoin mining requires a full node, protocol compatibility, network configuration, and secure payout systems.

## Features

- Worker registration
- Mining job generation
- Difficulty-based target checks
- Share submission and validation
- Pool-level summary statistics
- Simulated miner client

## Project structure

- `bitcoin_miner.py` – low-level block-header mining logic
- `mining_pool.py` – pool logic for jobs and share validation
- `worker.py` – example miner client
- `README.md` – project overview and usage

## Quick start

1. Create a virtual environment (optional but recommended):
   ```bash
   python3 -m venv .venv
   source .venv/bin/activate
   ```

2. Install dependencies:
   ```bash
   pip install -r requirements.txt
   ```

3. Run the demo miner:
   ```bash
   python worker.py
   ```

## Example output

```text
Pool: bc1qexample...
Worker demo-worker-01 connected.
Job created: 1
Mining job 1 assigned.
Share accepted for worker demo-worker-01. Share count: 1
Pool stats: {'total_shares': 1, 'blocks_found': 0, 'active_workers': 1}
```

## Security and production notes

A production mining pool requires:
- a proper Stratum protocol implementation
- secure worker authentication
- payout accounting and wallet integrations
- transaction validation and block templates
- monitoring, rate limiting, and failover
- a full Bitcoin node or compatible mining backend

## License

This project is provided for educational purposes only.
