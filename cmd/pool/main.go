package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gunasekaran1969/bitcoin-mining-pool/internal/pool"
)

func main() {
    poolAddress := "bc1qexamplepooladdress"
    if v := os.Getenv("POOL_ADDRESS"); v != "" {
        poolAddress = v
    }

    addr := ":3333"
    if v := os.Getenv("POOL_ADDR"); v != "" {
        addr = v
    }

    srv := pool.NewPool(poolAddress)
    log.Printf("Starting Bitcoin mining pool on %s", addr)
    log.Printf("Pool address: %s", poolAddress)

    go func() {
        if err := srv.ListenAndServe(addr); err != nil {
            log.Fatalf("pool server failed: %v", err)
        }
    }()

    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    <-sigs

    log.Println("Shutting down pool server...")
    time.Sleep(200 * time.Millisecond)
    fmt.Println("Pool stopped")
}
