package main

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
	inputDir   string
	outputDir  string
	presetFile string
)

func init() {
	flag.StringVar(&inputDir, "input", "./input", "Diretório de entrada com arquivos de vídeo")
	flag.StringVar(&outputDir, "output", "./output", "Diretório de saída para vídeos convertidos")
	flag.StringVar(&presetFile, "preset", "", "Arquivo de preset JSON para HandBrakeCLI")
}

func extractPercentage(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	// Regex para tempo estimado (HandBrakeCLI não fornece porcentagem diretamente)
	percentageRegex := regexp.MustCompile(`([0-9]+)%`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := percentageRegex.FindStringSubmatch(line); matches != nil {
			fmt.Printf("Current progress: %s%%\n", matches[1])
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading stderr: %v\n", err)
	}
}

func convertVideo(inputFile, outputDir, presetFile string, wg *sync.WaitGroup) {
	defer wg.Done()

	outputFile := filepath.Join(outputDir, filepath.Base(inputFile))
	outputFile = outputFile[:len(outputFile)-len(filepath.Ext(outputFile))] + "_converted.mp4"

	handbrakeCmd := []string{
		"HandBrakeCLI",
		"-i", inputFile,
		"-o", outputFile,
		"--preset-import-file", presetFile, // Usando o preset importado do arquivo JSON
	}

	cmd := exec.Command(handbrakeCmd[0], handbrakeCmd[1:]...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating stderr pipe: %v\n", err)
		return
	}

	fmt.Printf("Converting %s to %s with preset file %s\n", inputFile, outputFile, presetFile)
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting HandBrakeCLI: %v\n", err)
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
			go convertVideo(path, outputDir, presetFile, &wg)
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

	if presetFile == "" {
		fmt.Println("Error: No preset file specified. Use the -preset flag to specify a JSON preset file.")
		os.Exit(1)
	}

	if err := processDirectory(); err != nil {
		fmt.Printf("Error processing directory: %v\n", err)
	} else {
		fmt.Println("All conversions completed.")
	}
}

// ./video-converter -input /Users/leonanluppi/Downloads/media/in -output /Users/leonanluppi/Downloads/media/out -preset /Users/leonanluppi/Downloads/media/FC-preset.json
