#!/usr/bin/env python3
"""
Bitcoin Mining Implementation
Provides core mining functionality for the mining pool
"""

import hashlib
import time
import struct
from typing import Dict, Tuple, Optional
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class BlockHeader:
    """Represents a Bitcoin block header"""
    
    def __init__(self, version: int, prev_block_hash: bytes, merkle_root: bytes,
                 timestamp: int, bits: int, nonce: int = 0):
        self.version = version
        self.prev_block_hash = prev_block_hash
        self.merkle_root = merkle_root
        self.timestamp = timestamp
        self.bits = bits
        self.nonce = nonce
    
    def serialize(self) -> bytes:
        """Serialize block header to bytes"""
        header = struct.pack('<I', self.version)
        header += self.prev_block_hash
        header += self.merkle_root
        header += struct.pack('<I', self.timestamp)
        header += struct.pack('<I', self.bits)
        header += struct.pack('<I', self.nonce)
        return header
    
    def get_hash(self) -> bytes:
        """Calculate SHA256 hash of the block header (double SHA256)"""
        data = self.serialize()
        return hashlib.sha256(hashlib.sha256(data).digest()).digest()
    
    def get_hash_hex(self) -> str:
        """Get block hash as hex string (reversed for display)"""
        hash_bytes = self.get_hash()
        return hash_bytes[::-1].hex()


class DifficultyTarget:
    """Manages difficulty targets for mining"""
    
    @staticmethod
    def bits_to_target(bits: int) -> int:
        """Convert bits to target (difficulty)"""
        exponent = bits >> 24
        mantissa = bits & 0x00FFFFFF
        if exponent <= 3:
            return mantissa >> (8 * (3 - exponent))
        else:
            return mantissa << (8 * (exponent - 3))
    
    @staticmethod
    def target_to_bits(target: int) -> int:
        """Convert target to bits"""
        bits = target.bit_length()
        exponent = (bits + 7) // 8
        
        if exponent <= 3:
            mantissa = (target << (8 * (3 - exponent))) & 0xFFFFFF
        else:
            mantissa = (target >> (8 * (exponent - 3))) & 0xFFFFFF
        
        if mantissa & 0x800000:
            exponent += 1
            mantissa >>= 8
        
        return (exponent << 24) | mantissa
    
    @staticmethod
    def calculate_difficulty(bits: int) -> float:
        """Calculate difficulty multiplier"""
        max_target = 0x00000000FFFF0000000000000000000000000000000000000000000000000000
        target = DifficultyTarget.bits_to_target(bits)
        if target <= 0:
            return 0
        return max_target / target


class BitcoinMiner:
    """Main Bitcoin miner class"""
    
    def __init__(self, difficulty_bits: int = 0x207FFFFF):
        self.difficulty_bits = difficulty_bits
        self.target = DifficultyTarget.bits_to_target(difficulty_bits)
        self.difficulty = DifficultyTarget.calculate_difficulty(difficulty_bits)
        self.shares_found = 0
        self.hashes_computed = 0
        self.start_time = None
    
    def mine_block(self, version: int, prev_block_hash: bytes, merkle_root: bytes,
                   max_nonce: int = 2**32 - 1, difficulty_bits: Optional[int] = None) -> Tuple[Optional[int], int]:
        """
        Attempt to mine a block
        
        Args:
            version: Block version
            prev_block_hash: Previous block hash
            merkle_root: Merkle root of transactions
            max_nonce: Maximum nonce to try
            difficulty_bits: Optional difficulty override
        
        Returns:
            Tuple of (found_nonce, hashes_computed) or (None, hashes_computed) if not found
        """
        if difficulty_bits:
            self.target = DifficultyTarget.bits_to_target(difficulty_bits)
            self.difficulty_bits = difficulty_bits
        
        timestamp = int(time.time())
        self.start_time = time.time()
        nonce = 0
        self.hashes_computed = 0
        
        logger.info(f"Starting mining with difficulty: {self.difficulty:.2f}")
        logger.info(f"Target: {self.target:064x}")
        
        header = BlockHeader(version, prev_block_hash, merkle_root, timestamp, self.difficulty_bits, 0)
        
        while nonce <= max_nonce:
            header.nonce = nonce
            block_hash = header.get_hash()
            self.hashes_computed += 1
            
            # Check if hash meets difficulty target
            hash_int = int.from_bytes(block_hash, byteorder='big')
            if hash_int <= self.target:
                self.shares_found += 1
                elapsed = time.time() - self.start_time
                hashrate = self.hashes_computed / elapsed if elapsed > 0 else 0
                
                logger.info(f"✓ Block found!")
                logger.info(f"  Nonce: {nonce}")
                logger.info(f"  Hash: {header.get_hash_hex()}")
                logger.info(f"  Hashes computed: {self.hashes_computed:,}")
                logger.info(f"  Time elapsed: {elapsed:.2f}s")
                logger.info(f"  Hashrate: {hashrate/1e6:.2f} MH/s")
                
                return nonce, self.hashes_computed
            
            nonce += 1
            
            # Progress logging every 100M hashes
            if self.hashes_computed % 100_000_000 == 0:
                elapsed = time.time() - self.start_time
                hashrate = self.hashes_computed / elapsed if elapsed > 0 else 0
                logger.info(f"Progress: {self.hashes_computed:,} hashes, {hashrate/1e6:.2f} MH/s")
        
        elapsed = time.time() - self.start_time
        hashrate = self.hashes_computed / elapsed if elapsed > 0 else 0
        logger.info(f"Mining stopped. Total hashes: {self.hashes_computed:,}, Hashrate: {hashrate/1e6:.2f} MH/s")
        
        return None, self.hashes_computed
    
    def get_stats(self) -> Dict:
        """Get mining statistics"""
        return {
            'shares_found': self.shares_found,
            'difficulty': self.difficulty,
            'difficulty_bits': hex(self.difficulty_bits),
            'target': f'{self.target:064x}'
        }


class MiningPool:
    """Bitcoin Mining Pool Manager"""
    
    def __init__(self, pool_address: str, min_share_difficulty: float = 1.0):
        self.pool_address = pool_address
        self.min_share_difficulty = min_share_difficulty
        self.miners = {}
        self.shares = []
        self.blocks_found = 0
        self.total_shares = 0
    
    def register_miner(self, miner_id: str, miner: BitcoinMiner):
        """Register a miner with the pool"""
        self.miners[miner_id] = {
            'miner': miner,
            'shares': 0,
            'blocks': 0,
            'registered_at': time.time()
        }
        logger.info(f"Miner {miner_id} registered")
    
    def submit_share(self, miner_id: str, nonce: int, block_data: Dict) -> bool:
        """
        Submit a share from a miner
        
        Returns:
            True if share is valid and accepted
        """
        if miner_id not in self.miners:
            logger.warning(f"Unknown miner: {miner_id}")
            return False
        
        miner_info = self.miners[miner_id]
        miner_info['shares'] += 1
        self.total_shares += 1
        
        self.shares.append({
            'miner_id': miner_id,
            'nonce': nonce,
            'timestamp': time.time(),
            'block_data': block_data
        })
        
        logger.info(f"Share accepted from {miner_id} (Total: {self.total_shares})")
        return True
    
    def get_pool_stats(self) -> Dict:
        """Get pool statistics"""
        return {
            'pool_address': self.pool_address,
            'total_shares': self.total_shares,
            'blocks_found': self.blocks_found,
            'active_miners': len(self.miners),
            'miners': {
                miner_id: {
                    'shares': info['shares'],
                    'blocks': info['blocks']
                }
                for miner_id, info in self.miners.items()
            }
        }


if __name__ == '__main__':
    # Example usage
    miner = BitcoinMiner(difficulty_bits=0x207FFFFF)  # Easy difficulty for testing
    
    # Test mining
    prev_hash = bytes.fromhex('0000000000000000000000000000000000000000000000000000000000000000')
    merkle_root = bytes.fromhex('0000000000000000000000000000000000000000000000000000000000000000')
    
    print("Starting test mine...")
    nonce, hashes = miner.mine_block(1, prev_hash, merkle_root, max_nonce=1000000)
    
    if nonce:
        print(f"Success! Found block with nonce: {nonce}")
    else:
        print(f"No block found in {hashes:,} hashes")
