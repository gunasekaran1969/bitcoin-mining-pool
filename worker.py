import hashlib
import time
import uuid
from dataclasses import dataclass, field
from typing import Dict, List, Optional

from bitcoin_miner import BlockHeader, DifficultyTarget


@dataclass
class MiningJob:
    job_id: str
    prev_block_hash: bytes
    merkle_root: bytes
    version: int
    difficulty_bits: int
    start_nonce: int
    end_nonce: int
    timestamp: int


class MiningPool:
    def __init__(self, pool_address: str, difficulty_bits: int = 0x207FFFFF):
        self.pool_address = pool_address
        self.difficulty_bits = difficulty_bits
        self.workers: Dict[str, Dict] = {}
        self.jobs: Dict[str, MiningJob] = {}
        self.share_history: List[Dict] = []
        self.job_counter = 0
        self.total_shares = 0
        self.blocks_found = 0

    def register_worker(self, worker_id: str, username: str = "anonymous") -> Dict:
        self.workers[worker_id] = {
            "username": username,
            "shares": 0,
            "last_seen": time.time(),
        }
        return {"worker_id": worker_id, "status": "registered"}

    def create_job(self, prev_block_hash: Optional[bytes] = None, merkle_root: Optional[bytes] = None) -> MiningJob:
        self.job_counter += 1
        job_id = str(self.job_counter)

        if prev_block_hash is None:
            prev_block_hash = bytes.fromhex("00" * 32)

        if merkle_root is None:
            merkle_root = bytes.fromhex("11" * 32)

        job = MiningJob(
            job_id=job_id,
            prev_block_hash=prev_block_hash,
            merkle_root=merkle_root,
            version=1,
            difficulty_bits=self.difficulty_bits,
            start_nonce=0,
            end_nonce=2_000_000,
            timestamp=int(time.time()),
        )
        self.jobs[job_id] = job
        return job

    def get_job_for_worker(self, worker_id: str) -> Optional[MiningJob]:
        if worker_id not in self.workers:
            return None
        job = self.create_job()
        return job

    def submit_share(self, worker_id: str, job_id: str, nonce: int) -> Dict:
        if worker_id not in self.workers:
            return {"accepted": False, "reason": "unknown worker"}

        job = self.jobs.get(job_id)
        if job is None:
            return {"accepted": False, "reason": "unknown job"}

        header = BlockHeader(
            version=job.version,
            prev_block_hash=job.prev_block_hash,
            merkle_root=job.merkle_root,
            timestamp=job.timestamp,
            bits=job.difficulty_bits,
            nonce=nonce,
        )

        block_hash = header.get_hash()
        hash_int = int.from_bytes(block_hash, byteorder="big")
        target = DifficultyTarget.bits_to_target(job.difficulty_bits)
        is_valid = hash_int <= target

        if not is_valid:
            self.workers[worker_id]["last_seen"] = time.time()
            return {"accepted": False, "reason": "share below target", "hash_hex": header.get_hash_hex()}

        self.total_shares += 1
        self.workers[worker_id]["shares"] += 1
        self.share_history.append(
            {
                "worker_id": worker_id,
                "job_id": job_id,
                "nonce": nonce,
                "hash_hex": header.get_hash_hex(),
                "timestamp": time.time(),
            }
        )

        return {
            "accepted": True,
            "worker_id": worker_id,
            "job_id": job_id,
            "nonce": nonce,
            "hash_hex": header.get_hash_hex(),
            "total_shares": self.total_shares,
        }

    def get_stats(self) -> Dict:
        return {
            "pool_address": self.pool_address,
            "total_shares": self.total_shares,
            "blocks_found": self.blocks_found,
            "active_workers": len(self.workers),
            "workers": {
                worker_id: {"shares": info["shares"], "last_seen": info["last_seen"]}
                for worker_id, info in self.workers.items()
            },
        }


if __name__ == "__main__":
    pool = MiningPool(pool_address="bc1qexamplepooladdress")
    pool.register_worker("demo-worker-01", "demo-user")
    job = pool.get_job_for_worker("demo-worker-01")
    print("Job created:", job.job_id)

    for nonce in range(job.start_nonce, job.end_nonce):
        result = pool.submit_share("demo-worker-01", job.job_id, nonce)
        if result.get("accepted"):
            print("Accepted share:", result)
            break

    print(pool.get_stats())
