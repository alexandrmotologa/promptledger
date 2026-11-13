package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	fmt.Println("=== Starting promptledger Studio Capture & Asset Generation ===")

	port := 8095
	studioURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 1. Build promptledger binary with embedded UI
	fmt.Println("1. Building promptledger binary...")
	cmdBuild := exec.Command("go", "build", "-o", "bin/promptledger.exe", "./cmd/promptledger")
	cmdBuild.Dir = "B:/workgit/promptledger"
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		fmt.Printf("Build failed: %s\n", string(out))
		os.Exit(1)
	}
	fmt.Println("✓ Built bin/promptledger.exe")

	// 2. Start promptledger daemon in background
	fmt.Printf("2. Launching promptledger daemon on port %d...\n", port)
	tmpDataDir, _ := os.MkdirTemp("", "promptledger_capture_*")
	defer os.RemoveAll(tmpDataDir)

	daemonCmd := exec.Command("B:/workgit/promptledger/bin/promptledger.exe", "serve",
		"--prompts", "B:/workgit/promptledger/prompts",
		"--data", tmpDataDir,
		"--port", fmt.Sprintf("%d", port),
	)
	daemonCmd.Dir = "B:/workgit/promptledger"
	_ = daemonCmd.Start()
	defer func() {
		if daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()

	// Wait for daemon port
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ready := false
	for i := 0; i < 30; i++ {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !ready {
		fmt.Println("Error: promptledger daemon failed to start")
		os.Exit(1)
	}
	fmt.Printf("✓ Daemon is ready at %s\n", studioURL)

	// 3. Seed realistic traffic
	fmt.Println("3. Seeding production requests across all variants...")
	simulateReq := func(count int) {
		body, _ := json.Marshal(map[string]int{"count": count})
		_, _ = http.Post(fmt.Sprintf("%s/v1/simulate-traffic", studioURL), "application/json", bytes.NewReader(body))
	}

	simulateReq(35)
	time.Sleep(1500 * time.Millisecond) // Allow async shadow workers to process

	// Trigger a promotion and rollback to populate audit history
	_, _ = http.Post(fmt.Sprintf("%s/v1/prompts/checkout-intent-classifier/promote", studioURL), "application/json", bytes.NewReader([]byte(`{"version":"v1.3.0-canary"}`)))
	time.Sleep(200 * time.Millisecond)
	simulateReq(10)
	time.Sleep(800 * time.Millisecond)

	// 4. Capture Screenshots with Chrome Headless
	chromePath := `C:\Program Files\Google\Chrome\Application\chrome.exe`
	imagesDir := `B:/workgit/promptledger/docs/images`
	_ = os.MkdirAll(imagesDir, 0755)

	fmt.Println("4. Capturing authentic high-resolution screenshots with Chrome Headless...")

	captures := []struct {
		tab      string
		filename string
		width    int
		height   int
		delay    time.Duration
	}{
		{
			tab:      "overview",
			filename: "promptledger_overview.png",
			width:    1440,
			height:   900,
			delay:    800 * time.Millisecond,
		},
		{
			tab:      "registry",
			filename: "promptledger_prompt_registry.png",
			width:    1440,
			height:   900,
			delay:    800 * time.Millisecond,
		},
		{
			tab:      "routing",
			filename: "promptledger_traffic_routing.png",
			width:    1440,
			height:   900,
			delay:    800 * time.Millisecond,
		},
		{
			tab:      "shadow",
			filename: "promptledger_shadow_inspector.png",
			width:    1440,
			height:   900,
			delay:    800 * time.Millisecond,
		},
		{
			tab:      "gates",
			filename: "promptledger_safety_gates.png",
			width:    1440,
			height:   900,
			delay:    800 * time.Millisecond,
		},
		{
			tab:      "playground",
			filename: "promptledger_playground.png",
			width:    1440,
			height:   900,
			delay:    800 * time.Millisecond,
		},
		{
			tab:      "overview",
			filename: "promptledger_mobile.png",
			width:    390,
			height:   844,
			delay:    800 * time.Millisecond,
		},
	}

	for _, cap := range captures {
		outPath := filepath.Join(imagesDir, cap.filename)
		targetURL := fmt.Sprintf("%s/?tab=%s", studioURL, cap.tab)

		cmdChrome := exec.Command(chromePath,
			"--headless",
			"--disable-gpu",
			"--hide-scrollbars",
			fmt.Sprintf("--window-size=%d,%d", cap.width, cap.height),
			"--screenshot="+outPath,
			targetURL,
		)
		if out, err := cmdChrome.CombinedOutput(); err != nil {
			fmt.Printf("Error capturing %s: %s\n", cap.filename, string(out))
		} else {
			fmt.Printf("✓ Captured %s (%dx%d)\n", cap.filename, cap.width, cap.height)
		}
	}

	// 5. Generate Animated Demo GIF using ffmpeg
	fmt.Println("5. Generating animated demo GIF (promptledger_demo.gif)...")
	gifFramesDir, _ := os.MkdirTemp("", "promptledger_gif_frames_*")
	defer os.RemoveAll(gifFramesDir)

	gifTabs := []string{"overview", "registry", "routing", "shadow", "gates", "playground"}
	for idx, gTab := range gifTabs {
		framePath := filepath.Join(gifFramesDir, fmt.Sprintf("frame_%02d.png", idx))
		targetURL := fmt.Sprintf("%s/?tab=%s", studioURL, gTab)

		cmdFrame := exec.Command(chromePath,
			"--headless",
			"--disable-gpu",
			"--hide-scrollbars",
			"--window-size=1280,740",
			"--screenshot="+framePath,
			targetURL,
		)
		_ = cmdFrame.Run()
	}

	gifOutPath := filepath.Join(imagesDir, "promptledger_demo.gif")
	// Compile gif with ffmpeg with 1.4s delay per frame and optimal palette
	cmdFfmpeg := exec.Command("ffmpeg", "-y",
		"-framerate", "0.75",
		"-i", filepath.Join(gifFramesDir, "frame_%02d.png"),
		"-vf", "split[s0][s1];[s0]palettegen=max_colors=128:reserve_transparent=0[p];[s1][p]paletteuse=dither=bayer:bayer_scale=3",
		gifOutPath,
	)
	if out, err := cmdFfmpeg.CombinedOutput(); err != nil {
		fmt.Printf("Error generating demo GIF: %s\n", string(out))
	} else {
		fmt.Printf("✓ Generated animated demo GIF at %s\n", gifOutPath)
	}

	fmt.Println("=== Completed promptledger Authentic Asset Generation ===")
}

// ensure context package is used
var _ = context.Background
