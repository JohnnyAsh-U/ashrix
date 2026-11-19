package main

import (
	_ "net/http/pprof"
	"os"
)

// import "github.com/JohnnyAsh-U/ashrix-api/cmd/gateway/cli"

func main() {

	// go func() {
	// 	for {
	// 		var stats runtime.MemStats

	// 		runtime.ReadMemStats(&stats)

	// 		fmt.Printf("Goroutine=%d, stream=?, mem=%.1fMB sys=%.1fMB\n",
	// 			runtime.NumGoroutine(),
	// 			float64(stats.Alloc)/1024/1024,
	// 			float64(stats.Sys)/1024/1024,
	// 		)

	// 		fmt.Printf("Alloc: %d KB\n", stats.Alloc/(1024))
	// 		fmt.Printf("TotalAlloc: %d KB\n", stats.TotalAlloc/1024)
	// 		fmt.Printf("HeapAlloc: %d KB\n", stats.HeapAlloc/1024)
	// 		fmt.Printf("HeapSys: %d KB\n", stats.HeapSys/1024)
	// 		fmt.Printf("NumGC: %d\n", stats.NumGC)
	// 		time.Sleep(2 * time.Second)
	// 	}
	// }()

	// go http.ListenAndServe("localhost:6060", nil)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

}
