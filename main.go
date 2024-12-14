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
	outputDir := flag.String("output", "", "Diretório base onde será criada a pasta de saída com o sufixo '_CONV'")
	numWorkers := flag.Int("workers", 4, "Número de workers (threads) para processamento paralelo")
	flag.Parse()

	if *inputDir == "" || *outputDir == "" {
		fmt.Println("Uso: go run main.go -input <diretório de entrada> -output <diretório de saída> [-workers <número de workers>]")
		return
	}

	inputBase := filepath.Base(*inputDir)
	outputBase := filepath.Join(*outputDir, inputBase+"_CONV")

	if _, err := os.Stat(outputBase); os.IsNotExist(err) {
		if err := os.MkdirAll(outputBase, os.ModePerm); err != nil {
			fmt.Printf("Erro ao criar diretório de saída: %v\n", err)
			return
		}
	}

	var files []string

	err := filepath.Walk(*inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(*inputDir, path)
		if err != nil {
			return err
		}

		outputPath := filepath.Join(outputBase, relativePath)
		if info.IsDir() {
			if err := os.MkdirAll(outputPath, os.ModePerm); err != nil {
				return fmt.Errorf("erro ao criar diretório %s: %v", outputPath, err)
			}
		} else if filepath.Ext(path) == ".mp4" {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Erro ao espelhar estrutura de diretórios: %v\n", err)
		return
	}

	totalFiles := len(files)
	if totalFiles == 0 {
		fmt.Println("Nenhum arquivo .mp4 encontrado no diretório de entrada.")
		return
	}

	startTotal := time.Now()

	var wg sync.WaitGroup
	sem := make(chan struct{}, *numWorkers) // Número máximo de goroutines simultâneas

	for _, inputFile := range files {
		wg.Add(1)
		sem <- struct{}{}

		go func(inputFile string) {
			defer wg.Done()
			defer func() { <-sem }() // Libera o worker

			relativePath, err := filepath.Rel(*inputDir, inputFile)
			if err != nil {
				fmt.Printf("Erro ao obter caminho relativo: %v\n", err)
				return
			}

			outputFile := filepath.Join(outputBase, relativePath)
			outputDirPath := filepath.Dir(outputFile)

			if err := os.MkdirAll(outputDirPath, os.ModePerm); err != nil {
				fmt.Printf("Erro ao criar diretórios de saída: %v\n", err)
				return
			}

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
