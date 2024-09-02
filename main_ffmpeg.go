package main

//12 minutos
import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
)

var (
	inputDir  string
	outputDir string
)

func init() {
	flag.StringVar(&inputDir, "input", "./input", "Diretório de entrada com arquivos de vídeo")
	flag.StringVar(&outputDir, "output", "./output", "Diretório de saída para vídeos convertidos")
}

func extractPercentage(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	percentageRegex := regexp.MustCompile(`time=(\d+:\d+:\d+.\d+)`)

	var lastTime string
	for scanner.Scan() {
		line := scanner.Text()
		if matches := percentageRegex.FindStringSubmatch(line); matches != nil {
			lastTime = matches[1]
			fmt.Printf("Current progress: %s\n", lastTime)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading stderr: %v\n", err)
	}
}

func convertVideo(inputFile, outputDir string, wg *sync.WaitGroup) {
	defer wg.Done()

	outputFile := filepath.Join(outputDir, filepath.Base(inputFile))
	outputFile = outputFile[:len(outputFile)-len(filepath.Ext(outputFile))] + "_converted.mp4"

	ffmpegCmd := []string{
		"ffmpeg",
		"-i", inputFile,
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", "fast",
		"-c:a", "aac",
		"-b:a", "128k",
		"-threads", "0",
		outputFile,
	}

	cmd := exec.Command(ffmpegCmd[0], ffmpegCmd[1:]...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating stderr pipe: %v\n", err)
		return
	}

	fmt.Printf("Converting %s to %s\n", inputFile, outputFile)
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting ffmpeg: %v\n", err)
		return
	}

	go extractPercentage(stderr)

	err = cmd.Wait()
	if err != nil {
		fmt.Printf("Error converting %s: %v\n", inputFile, err)
		return
	}
	fmt.Printf("Conversion completed: %s\n", outputFile)
}

func processDirectory() error {
	var wg sync.WaitGroup

	err := filepath.WalkDir(inputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && filepath.Ext(path) == ".mp4" {
			wg.Add(1)
			go convertVideo(path, outputDir, &wg)
		}
		return nil
	})

	if err != nil {
		return err
	}

	wg.Wait()
	return nil
}

func main() {
	flag.Parse()

	if err := processDirectory(); err != nil {
		fmt.Printf("Error processing directory: %v\n", err)
	} else {
		fmt.Println("All conversions completed.")
	}
}

//./video-converter -input /Users/leonanluppi/Downloads/media/in -output /Users/leonanluppi/Downloads/media/out
