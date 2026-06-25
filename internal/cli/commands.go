package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

func runWait(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: rsk wait <seconds>\n")
		os.Exit(2)
	}
	n, _ := strconv.Atoi(args[0])
	if n <= 0 {
		return
	}
	time.Sleep(time.Duration(n) * time.Second)
}

func runEnv() {
	fmt.Printf("RSK_SERVER=%s\n", resolveServerURL())
	token := resolveToken()
	masked := token
	if len(masked) > 8 {
		masked = masked[:8] + "..." + masked[len(masked)-4:]
	}
	fmt.Printf("RSK_TOKEN=%s\n", masked)
	if v := os.Getenv("RSK_DEVICE"); v != "" {
		fmt.Printf("RSK_DEVICE=%s\n", v)
	}
}

func runDevices(serverURL, token string) {
	resp, err := sendRequest(serverURL, token, "", "devices", nil, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if resp.Final != nil {
		b, _ := json.MarshalIndent(resp.Final, "", "  ")
		fmt.Println(string(b))
	}
}
