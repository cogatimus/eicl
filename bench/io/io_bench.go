// parallelized disk r/w benchmarking
// customizable block size, file size, thread count
// optional CPU profiling, auto-opens HTML report

package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

type BenchmarkResult struct {
	FileSizeMB       int
	BlockSizeKB      int
	Threads          int
	EnablePprof      bool
	WriteSpeedMB     float64
	ReadSpeedMB      float64
	AllocBefore      float64
	AllocAfter       float64
	TotalAllocBefore float64
	TotalAllocAfter  float64
}

var (
	fileSizeMB  int
	blockSizeKB int
	threads     int
	enablePprof bool
	tempFile    = "iobench_tempfile.tmp"
)

func init() {
	flag.IntVar(&fileSizeMB, "filesize", 256, "Total file size in MB")
	flag.IntVar(&blockSizeKB, "blocksize", 128, "Block size per operation in KB")
	flag.IntVar(&threads, "threads", 4, "Number of concurrent threads")
	flag.BoolVar(&enablePprof, "pprof", false, "Enable CPU profiling")
}

func main() {
	flag.Parse()

	memBefore := captureMemStats()
	startPprof()

	writeSpeed := runBenchmark(diskWriteWorker)
	readSpeed := runBenchmark(diskReadWorker)

	stopPprof()
	memAfter := captureMemStats()

	result := BenchmarkResult{
		FileSizeMB:       fileSizeMB,
		BlockSizeKB:      blockSizeKB,
		Threads:          threads,
		EnablePprof:      enablePprof,
		WriteSpeedMB:     writeSpeed,
		ReadSpeedMB:      readSpeed,
		AllocBefore:      float64(memBefore.Alloc) / (1024 * 1024),
		AllocAfter:       float64(memAfter.Alloc) / (1024 * 1024),
		TotalAllocBefore: float64(memBefore.TotalAlloc) / (1024 * 1024),
		TotalAllocAfter:  float64(memAfter.TotalAlloc) / (1024 * 1024),
	}

	_ = os.Remove(tempFile)
	generateHTMLReport(result)
	fmt.Println("Benchmark complete. Report written to: output.html")
}

func runBenchmark(worker func(int)) float64 {
	totalBytes := fileSizeMB * 1024 * 1024
	perThreadBytes := totalBytes / threads
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(perThreadBytes)
		}()
	}
	wg.Wait()
	duration := time.Since(start).Seconds()
	return float64(totalBytes) / (1024 * 1024) / duration
}

func diskWriteWorker(bytes int) {
	f, _ := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	defer f.Close()
	block := make([]byte, blockSizeKB*1024)
	for written := 0; written < bytes; {
		n, _ := f.Write(block)
		written += n
	}
}

func diskReadWorker(bytes int) {
	f, _ := os.Open(tempFile)
	defer f.Close()
	block := make([]byte, blockSizeKB*1024)
	for read := 0; read < bytes; {
		n, err := f.Read(block)
		if err != nil && err != io.EOF {
			break
		}
		if n == 0 {
			break
		}
		read += n
	}
}

func captureMemStats() runtime.MemStats {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return mem
}

func startPprof() {
	if !enablePprof {
		return
	}
	f, _ := os.Create("cpu.pprof")
	_ = pprof.StartCPUProfile(f)
}

func stopPprof() {
	if enablePprof {
		pprof.StopCPUProfile()
	}
}

func generateHTMLReport(result BenchmarkResult) {
	const tpl = `
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>I/O Benchmark Report</title>
	<style>
		body { font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 30px; }
		h1 { color: #333; }
		table { border-collapse: collapse; width: 60%; background-color: white; }
		th, td { border: 1px solid #ccc; padding: 10px 15px; text-align: left; }
		th { background-color: #eee; }
	</style>
</head>
<body>
	<h1>I/O Benchmark Report</h1>
	<table>
		<tr><th>File Size (MB)</th><td>{{.FileSizeMB}}</td></tr>
		<tr><th>Block Size (KB)</th><td>{{.BlockSizeKB}}</td></tr>
		<tr><th>Threads</th><td>{{.Threads}}</td></tr>
		<tr><th>CPU Profiling</th><td>{{.EnablePprof}}</td></tr>
		<tr><th>Write Speed (MB/s)</th><td>{{printf "%.2f" .WriteSpeedMB}}</td></tr>
		<tr><th>Read Speed (MB/s)</th><td>{{printf "%.2f" .ReadSpeedMB}}</td></tr>
		<tr><th>Alloc Before (MB)</th><td>{{printf "%.2f" .AllocBefore}}</td></tr>
		<tr><th>Alloc After (MB)</th><td>{{printf "%.2f" .AllocAfter}}</td></tr>
		<tr><th>Total Alloc Before (MB)</th><td>{{printf "%.2f" .TotalAllocBefore}}</td></tr>
		<tr><th>Total Alloc After (MB)</th><td>{{printf "%.2f" .TotalAllocAfter}}</td></tr>
	</table>
</body>
</html>`

	t, _ := template.New("report").Parse(tpl)
	f, _ := os.Create("output.html")
	defer f.Close()
	t.Execute(f, result)
	openInBrowser("output.html")
}

func openInBrowser(file string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", file)
	case "darwin":
		cmd = exec.Command("open", file)
	default:
		cmd = exec.Command("xdg-open", file)
	}
	cmd.Start()
}
