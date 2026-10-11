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
    Timestamp uint32
    Target    *big.Int
}

type Worker struct {
    ID       string
    Username string
    Shares   uint64
    LastSeen time.Time
}

type Pool struct {
    mu          sync.Mutex
    poolAddress string
    workers     map[string]*Worker
    jobs        map[string]*Job
    totalShares uint64
    jobCounter  uint64
}

type stratumRequest struct {
    ID     any                 `json:"id"`
    Method string              `json:"method"`
    Params []json.RawMessage   `json:"params"`
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

    log.Printf("Pool listening on %s", addr)

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
                    []any{"mining.notify", "pool-demo"},
                    "0",
                },
                Error: nil,
            }
            if err := json.NewEncoder(conn).Encode(resp); err != nil {
                return
            }

            notify := stratumNotification{
                Method: "mining.notify",
                Params: []any{
                    job.ID,
                    hex.EncodeToString(job.PrevHash[:]),
                    hex.EncodeToString(job.MerkleRoot[:]),
                    "00000000",
                    fmt.Sprintf("%08x", job.Timestamp),
                    "00000000",
                },
            }
            if err := json.NewEncoder(conn).Encode(notify); err != nil {
                return
            }

        case "mining.authorize":
            if len(req.Params) == 0 {
                sendError(conn, req.ID, "missing worker")
                continue
            }
            workerID := strings.Trim(string(req.Params[0]), "\"")
            p.registerWorker(workerID)
            resp := stratumResponse{ID: req.ID, Result: true, Error: nil}
            if err := json.NewEncoder(conn).Encode(resp); err != nil {
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

            result := p.validateShare(workerID, jobID, uint32(nonce))
            resp := stratumResponse{ID: req.ID, Result: result.Accepted, Error: nil}
            if !result.Accepted {
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

func (p *Pool) registerWorker(workerID string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    if _, ok := p.workers[workerID]; ok {
        return
    }
    p.workers[workerID] = &Worker{
        ID:       workerID,
        Username: workerID,
        LastSeen: time.Now(),
    }
    log.Printf("worker registered: %s", workerID)
}

func (p *Pool) createJob() *Job {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.jobCounter++
    prevHash := sha256.Sum256([]byte(fmt.Sprintf("prev-%d-%d", p.jobCounter, time.Now().UnixNano())))
    merkleRoot := sha256.Sum256([]byte(fmt.Sprintf("merkle-%d-%d", p.jobCounter, time.Now().UnixNano())))

    target := new(big.Int)
    target.SetBytes([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})

    job := &Job{
        ID:        fmt.Sprintf("job-%d", p.jobCounter),
        PrevHash:  prevHash,
        MerkleRoot: merkleRoot,
        Timestamp: uint32(time.Now().Unix()),
        Target:    target,
    }
    p.jobs[job.ID] = job
    return job
}

func (p *Pool) validateShare(workerID, jobID string, nonce uint32) ShareResult {
    p.mu.Lock()
    defer p.mu.Unlock()

    if _, ok := p.workers[workerID]; !ok {
        return ShareResult{WorkerID: workerID, JobID: jobID, Accepted: false, Reason: "unknown worker"}
    }

    job, ok := p.jobs[jobID]
    if !ok {
        return ShareResult{WorkerID: workerID, JobID: jobID, Accepted: false, Reason: "unknown job"}
    }

    header := make([]byte, 80)
    binary.LittleEndian.PutUint32(header[0:4], 1)
    copy(header[4:36], job.PrevHash[:])
    copy(header[36:68], job.MerkleRoot[:])
    binary.LittleEndian.PutUint32(header[68:72], job.Timestamp)
    binary.LittleEndian.PutUint32(header[72:76], 0x0300ffff)
    binary.LittleEndian.PutUint32(header[76:80], nonce)

    h1 := sha256.Sum256(header)
    h2 := sha256.Sum256(h1[:])
    hashInt := new(big.Int).SetBytes(h2[:])

    if hashInt.Cmp(job.Target) <= 0 {
        p.totalShares++
        p.workers[workerID].Shares++
        p.workers[workerID].LastSeen = time.Now()
        return ShareResult{
            WorkerID: workerID,
            JobID:    jobID,
            Nonce:    nonce,
            HashHex:  hex.EncodeToString(h2[:]),
            Accepted: true,
        }
    }

    return ShareResult{
        WorkerID: workerID,
        JobID:    jobID,
        Nonce:    nonce,
        HashHex:  hex.EncodeToString(h2[:]),
        Accepted: false,
        Reason:   "share below target",
    }
}

type ShareResult struct {
    WorkerID string
    JobID    string
    Nonce    uint32
    HashHex  string
    Accepted bool
    Reason   string
}

func (p *Pool) Stats() map[string]any {
    p.mu.Lock()
    defer p.mu.Unlock()

    return map[string]any{
        "pool_address": p.poolAddress,
        "total_shares": p.totalShares,
        "active_workers": len(p.workers),
    }
}
