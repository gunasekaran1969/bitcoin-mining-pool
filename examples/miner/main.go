package pool

import (
    "bufio"
    "crypto/sha256"
    "encoding/binary"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "math/big"
    "net"
    "strconv"
    "strings"
    "sync"
    "time"
)

type Job struct {
    ID        string
    PrevHash  [32]byte
    MerkleRoot [32]byte
    Version   uint32
    Bits      uint32
    Timestamp uint32
    Nonce     uint32
}

type Worker struct {
    ID       string
    Username string
    Shares   uint64
    LastSeen time.Time
}

type ShareResult struct {
    JobID    string
    WorkerID string
    Nonce    uint32
    HashHex  string
    Accepted bool
    Reason   string
}

type Stats struct {
    PoolAddress string
    TotalShares uint64
    ActiveWorkers int
    BlocksFound int
}

type Pool struct {
    mu           sync.Mutex
    poolAddress  string
    workers      map[string]*Worker
    jobs         map[string]*Job
    totalShares  uint64
    blocksFound  int
    jobSequence  uint64
}

type stratumRequest struct {
    ID     any    `json:"id"`
    Method string `json:"method"`
    Params []json.RawMessage `json:"params"`
}

type stratumResponse struct {
    ID     any  `json:"id"`
    Result any  `json:"result"`
    Error  any  `json:"error"`
}

type stratumNotification struct {
    Method string `json:"method"`
    Params []any  `json:"params"`
}

func NewPool(poolAddress string) *Pool {
    return &Pool{
        poolAddress: poolAddress,
        workers:     make(map[string]*Worker),
        jobs:        make(map[string]*Job),
    }
}

func (p *Pool) ListenAndServe(addr string) error {
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }
    defer ln.Close()

    for {
        conn, err := ln.Accept()
        if err != nil {
            log.Printf("accept error: %v", err)
            continue
        }
        go p.handleConn(conn)
    }
}

func (p *Pool) handleConn(conn net.Conn) {
    defer conn.Close()

    reader := bufio.NewReader(conn)
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            return
        }

        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        var req stratumRequest
        if err := json.Unmarshal([]byte(line), &req); err != nil {
            sendError(conn, req.ID, fmt.Sprintf("bad request: %v", err))
            continue
        }

        switch req.Method {
        case "mining.subscribe":
            job := p.createJob()
            resp := stratumResponse{
                ID: req.ID,
                Result: []any{
                    []any{"mining.notify", "bitcoin-pool-go"},
                    "00000000",
                },
                Error: nil,
            }
            if err := json.NewEncoder(conn).Encode(resp); err != nil {
                return
            }
            notify := stratumNotification{
                Method: "mining.notify",
                Params: []any{job.ID, hex.EncodeToString(job.PrevHash[:]), hex.EncodeToString(job.MerkleRoot[:]), fmt.Sprintf("%08x", job.Bits), fmt.Sprintf("%08x", job.Timestamp), "00000000"},
            }
            if err := json.NewEncoder(conn).Encode(notify); err != nil {
                return
            }
        case "mining.authorize":
            workerID := string(req.Params[0])
            worker := p.getOrCreateWorker(workerID)
            worker.LastSeen = time.Now()
            resp := stratumResponse{ID: req.ID, Result: true, Error: nil}
            if err := json.NewEncoder(conn).Encode(resp); err != nil {
                return
            }
            job := p.createJob()
            notify := stratumNotification{
                Method: "mining.notify",
                Params: []any{job.ID, hex.EncodeToString(job.PrevHash[:]), hex.EncodeToString(job.MerkleRoot[:]), fmt.Sprintf("%08x", job.Bits), fmt.Sprintf("%08x", job.Timestamp), "00000000"},
            }
            if err := json.NewEncoder(conn).Encode(notify); err != nil {
                return
            }
        case "mining.submit":
            if len(req.Params) < 3 {
                sendError(conn, req.ID, "missing submit params")
                continue
            }

            workerID := strings.Trim(string(req.Params[0]), "\"")
            jobID := strings.Trim(string(req.Params[1]), "\"")
            nonceText := strings.Trim(string(req.Params[2]), "\"")
            nonce, err := strconv.ParseUint(nonceText, 16, 32)
            if err != nil {
                sendError(conn, req.ID, fmt.Sprintf("invalid nonce: %v", err))
                continue
            }

            result := p.ValidateShare(workerID, jobID, uint32(nonce))
            resp := stratumResponse{ID: req.ID, Result: result.Accepted, Error: nil}
            if !result.Accepted {
                resp.Result = false
                resp.Error = result.Reason
            }
            if err := json.NewEncoder(conn).Encode(resp); err != nil {
                return
            }
        default:
            sendError(conn, req.ID, fmt.Sprintf("unsupported method: %s", req.Method))
        }
    }
}

func sendError(conn net.Conn, id any, msg string) {
    resp := stratumResponse{ID: id, Result: nil, Error: msg}
    _ = json.NewEncoder(conn).Encode(resp)
}

func (p *Pool) createJob() *Job {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.jobSequence++
    seed := fmt.Sprintf("pool:%s:%d", p.poolAddress, p.jobSequence)
    prevHash := sha256.Sum256([]byte(seed))
    merkleRoot := sha256.Sum256([]byte("merkle:" + seed))

    job := &Job{
        ID:         fmt.Sprintf("job-%d", p.jobSequence),
        PrevHash:   prevHash,
        MerkleRoot: merkleRoot,
        Version:    1,
        Bits:       0x1900ffff,
        Timestamp:  uint32(time.Now().Unix()),
        Nonce:      0,
    }
    p.jobs[job.ID] = job
    return job
}

func (p *Pool) getOrCreateWorker(workerID string) *Worker {
    p.mu.Lock()
    defer p.mu.Unlock()

    if w, ok := p.workers[workerID]; ok {
        return w
    }

    w := &Worker{ID: workerID, Username: workerID, LastSeen: time.Now()}
    p.workers[workerID] = w
    return w
}

func (p *Pool) ValidateShare(workerID, jobID string, nonce uint32) ShareResult {
    p.mu.Lock()
    defer p.mu.Unlock()

    worker, ok := p.workers[workerID]
    if !ok {
        return ShareResult{WorkerID: workerID, JobID: jobID, Accepted: false, Reason: "unknown worker"}
    }

    job, ok := p.jobs[jobID]
    if !ok {
        return ShareResult{WorkerID: workerID, JobID: jobID, Accepted: false, Reason: "unknown job"}
    }

    header := make([]byte, 80)
    binary.LittleEndian.PutUint32(header[0:4], job.Version)
    copy(header[4:36], job.PrevHash[:])
    copy(header[36:68], job.MerkleRoot[:])
    binary.LittleEndian.PutUint32(header[68:72], job.Timestamp)
    binary.LittleEndian.PutUint32(header[72:76], job.Bits)
    binary.LittleEndian.PutUint32(header[76:80], nonce)

    h1 := sha256.Sum256(header)
    h2 := sha256.Sum256(h1[:])
    hashInt := new(big.Int).SetBytes(h2[:])
    target := bitsToTarget(job.Bits)

    accepted := hashInt.Cmp(target) <= 0
    if accepted {
        worker.Shares++
        p.totalShares++
        worker.LastSeen = time.Now()
        return ShareResult{
            JobID: jobID,
            WorkerID: workerID,
            Nonce: nonce,
            HashHex: hex.EncodeToString(h2[:]),
            Accepted: true,
        }
    }

    return ShareResult{
        JobID: jobID,
        WorkerID: workerID,
        Nonce: nonce,
        HashHex: hex.EncodeToString(h2[:]),
        Accepted: false,
        Reason: "below target",
    }
}

func (p *Pool) Stats() Stats {
    p.mu.Lock()
    defer p.mu.Unlock()
    return Stats{
        PoolAddress: p.poolAddress,
        TotalShares: p.totalShares,
        ActiveWorkers: len(p.workers),
        BlocksFound: p.blocksFound,
    }
}

func bitsToTarget(bits uint32) *big.Int {
    exponent := int(bits >> 24)
    mantissa := bits & 0x00ffffff

    target := new(big.Int).SetUint64(uint64(mantissa))
    if exponent <= 3 {
        shift := uint(8 * (3 - exponent))
        return target.Rsh(target, shift)
    }

    shift := uint(8 * (exponent - 3))
    return target.Lsh(target, shift)
}
