package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func convertVideo(inputFile, outputDir, presetFile string) {
	outputFile := filepath.Join(outputDir, filepath.Base(inputFile))
	outputFile = outputFile[:len(outputFile)-len(filepath.Ext(outputFile))] + "_converted.mp4"

	handbrakeCmd := []string{
		"HandBrakeCLI",
		"-i", inputFile,
		"-o", outputFile,
		"--preset-import-file", presetFile,
		"--encoder", "x264",
		"--encoder-preset", "veryfast", // Ajuste o preset conforme necessário
	}

	cmd := exec.Command(handbrakeCmd[0], handbrakeCmd[1:]...)
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error converting %s: %v\n", inputFile, err)
		return
	}
	fmt.Printf("Conversion completed: %s\n", outputFile)
}

func processDirectory() error {
	err := filepath.WalkDir(inputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && filepath.Ext(path) == ".mp4" {
			convertVideo(path, outputDir, presetFile)
		}
		return nil
	})

	return err
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
