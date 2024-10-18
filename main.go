package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

func main() {
	inputDir := flag.String("input", "", "Diretório de entrada contendo arquivos .mp4")
	outputDir := flag.String("output", "", "Diretório de saída para os arquivos convertidos")
	flag.Parse()

	if *inputDir == "" || *outputDir == "" {
		fmt.Println("Uso: go run main.go -input <diretório de entrada> -output <diretório de saída>")
		return
	}

	if _, err := os.Stat(*outputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(*outputDir, os.ModePerm); err != nil {
			fmt.Printf("Erro ao criar diretório de saída: %v\n", err)
			return
		}
	}

	files, err := filepath.Glob(filepath.Join(*inputDir, "*.mp4"))
	if err != nil {
		fmt.Printf("Erro ao listar arquivos .mp4: %v\n", err)
		return
	}

	totalFiles := len(files)
	if totalFiles == 0 {
		fmt.Println("Nenhum arquivo .mp4 encontrado no diretório de entrada.")
		return
	}

	startTotal := time.Now()

	var wg sync.WaitGroup
	// Define o número máximo de goroutines simultâneas
	maxWorkers := 4 
	sem := make(chan struct{}, maxWorkers)

	for _, inputFile := range files {
		wg.Add(1)
		sem <- struct{}{}

		go func(inputFile string) {
			defer wg.Done()
			// Libera o worker
			defer func() { <-sem }() 

			outputFile := filepath.Join(*outputDir, filepath.Base(inputFile))
			// outputFile = strings.Replace(outputFile, ".mp4", "_converted.mp4", 1)

			start := time.Now()
			fmt.Printf("Iniciando conversão de %s em %s\n", inputFile, start.Format("15:04:05"))

			cmd := exec.Command("ffmpeg", "-i", inputFile, "-c:v", "libx264", "-movflags", "faststart", "-crf", "30", "-preset", "superfast", outputFile)
			cmd.Stderr = nil
			cmd.Stdout = nil

			if err := cmd.Run(); err != nil {
				fmt.Printf("Erro ao converter %s: %v\n", inputFile, err)
			} else {
				duration := time.Since(start)
				fmt.Printf("Conversão de %s concluída em %s. Tempo decorrido: %v\n", inputFile, time.Now().Format("15:04:05"), duration)
			}
		}(inputFile)
	}

	wg.Wait() 
	totalDuration := time.Since(startTotal)
	fmt.Printf("\nTodas as conversões foram concluídas. Tempo total: %v\n", totalDuration)
}
