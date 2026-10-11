package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "log"
    "net"
    "os"
    "time"
)

type request struct {
    ID     any                 `json:"id"`
    Method string              `json:"method"`
    Params []json.RawMessage   `json:"params"`
}

type response struct {
    ID     any  `json:"id"`
    Result any  `json:"result"`
    Error  any  `json:"error"`
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
    line, err := reader.ReadString('\n')
    if err != nil {
        log.Fatalf("read subscribe response: %v", err)
    }
    fmt.Println("subscribe response:", line)

    send(map[string]any{"id": 2, "method": "mining.authorize", "params": []any{"miner-demo", "password"}})
    line, err = reader.ReadString('\n')
    if err != nil {
        log.Fatalf("read authorize response: %v", err)
    }
    fmt.Println("authorize response:", line)

    line, err = reader.ReadString('\n')
    if err != nil {
        log.Fatalf("read job notify: %v", err)
    }
    fmt.Println("job notify:", line)

    // Simple share submission to the pool.
    time.Sleep(200 * time.Millisecond)
    send(map[string]any{"id": 3, "method": "mining.submit", "params": []any{"miner-demo", "job-1", "00000001"}})
    resultLine, err := reader.ReadString('\n')
    if err != nil {
        log.Fatalf("read submit response: %v", err)
    }
    fmt.Println("submit response:", resultLine)
}
