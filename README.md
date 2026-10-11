package main

import (
    "bufio"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "math/big"
    "net"
    "os"
    "strconv"
    "strings"
    "time"

    "crypto/sha256"
)

type request struct {
    ID     any    `json:"id"`
    Method string `json:"method"`
    Params []json.RawMessage `json:"params"`
}

type response struct {
    ID     any  `json:"id"`
    Result any  `json:"result"`
    Error  any  `json:"error"`
}

type notify struct {
    Method string `json:"method"`
    Params []any  `json:"params"`
}

func main() {
    host := os.Getenv("POOL_ADDR")
    if host == "" {
        host = "127.0.0.1:3333"
    }

    conn, err := net.Dial("tcp", host)
    if err != nil {
        log.Fatalf("connect to pool: %v", err)
    }
    defer conn.Close()

    reader := bufio.NewReader(conn)

    send := func(req any) {
        if err := json.NewEncoder(conn).Encode(req); err != nil {
            log.Fatalf("send request: %v", err)
        }
    }

    send(map[string]any{"id": 1, "method": "mining.subscribe", "params": []any{}})
    line, _ := reader.ReadString('\n')
    var sub response
    _ = json.Unmarshal([]byte(line), &sub)
    fmt.Println("Subscribed:", line)

    send(map[string]any{"id": 2, "method": "mining.authorize", "params": []any{"miner-demo", "password"}})
    authLine, _ := reader.ReadString('\n')
    fmt.Println("Authorized:", authLine)

    // read the mining.notify
    notifyLine, _ := reader.ReadString('\n')
    fmt.Println("Job notify:", notifyLine)

    // demo miner loops until it finds a likely share
    var job notify
    if err := json.Unmarshal([]byte(notifyLine), &job); err == nil {
        if len(job.Params) >= 6 {
            jobID := job.Params[0].(string)
            prevHashHex := job.Params[1].(string)
            merkleHex := job.Params[2].(string)
            bitsStr := job.Params[3].(string)
            bits, _ := strconv.ParseUint(bitsStr, 16, 32)
            timeStampStr := job.Params[4].(string)
            timeStamp, _ := strconv.ParseUint(timeStampStr, 16, 32)
            _ = prevHashHex
            _ = merkleHex
            _ = timeStamp

            target := bitsToTarget(uint32(bits))
            for nonce := uint32(0); nonce < 1_000_000; nonce++ {
                header := make([]byte, 80)
                binaryLittlePutUint32(header[0:4], 1)
                prev, _ := hex.DecodeString(prevHashHex)
                copy(header[4:36], prev)
                merkle, _ := hex.DecodeString(merkleHex)
                copy(header[36:68], merkle)
                binaryLittlePutUint32(header[68:72], uint32(timeStamp))
                binaryLittlePutUint32(header[72:76], uint32(bits))
                binaryLittlePutUint32(header[76:80], nonce)

                h1 := sha256.Sum256(header)
                h2 := sha256.Sum256(h1[:])
                hInt := new(big.Int).SetBytes(h2[:])
                if hInt.Cmp(target) <= 0 {
                    fmt.Printf("Found share for job=%s nonce=%d hash=%s\n", jobID, nonce, hex.EncodeToString(h2[:]))
                    send(map[string]any{"id": 3, "method": "mining.submit", "params": []any{"miner-demo", jobID, fmt.Sprintf("%08x", nonce)}})
                    resultLine, _ := reader.ReadString('\n')
                    fmt.Println("Submit result:", resultLine)
                    return
                }
            }
        }
    }

    time.Sleep(500 * time.Millisecond)
    send(map[string]any{"id": 3, "method": "mining.submit", "params": []any{"miner-demo", "job-1", "00000000"}})
    resultLine, _ := reader.ReadString('\n')
    fmt.Println("Submit result:", resultLine)
}

func binaryLittlePutUint32(dst []byte, value uint32) {
    binary := make([]byte, 4)
    binary[0] = byte(value)
    binary[1] = byte(value >> 8)
    binary[2] = byte(value >> 16)
    binary[3] = byte(value >> 24)
    copy(dst, binary)
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
