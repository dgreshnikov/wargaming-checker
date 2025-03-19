package resultmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type ResultManager struct {
	results                []Result
	badFile                *os.File
	errorsFile             *os.File
	goodFile               *os.File
	goodNoInfoFile         *os.File
	tier10File             *os.File
	mergeRequiredFile      *os.File
	tier10NoInfoFile       *os.File
	realmFiles             map[string]*os.File
	realmNoInfoFiles       map[string]*os.File
	realmTier10Files       map[string]*os.File
	realmTier10NoInfoFiles map[string]*os.File
	outputPath             string

	mu sync.Mutex
}

func NewResultManager(outputPath string) (*ResultManager, error) {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	badFile, err := os.OpenFile(filepath.Join(outputPath, "bad.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open bad.txt: %w", err)
	}

	goodFile, err := os.OpenFile(filepath.Join(outputPath, "good.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		badFile.Close()
		return nil, fmt.Errorf("failed to open good.txt: %w", err)
	}

	goodNoInfoFile, err := os.OpenFile(filepath.Join(outputPath, "good_no_info.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		badFile.Close()
		goodFile.Close()
		return nil, fmt.Errorf("failed to open good_no_info.txt: %w", err)
	}

	tier10File, err := os.OpenFile(filepath.Join(outputPath, "tier10.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		badFile.Close()
		goodFile.Close()
		goodNoInfoFile.Close()
		return nil, fmt.Errorf("failed to open tier10.txt: %w", err)
	}

	tier10NoInfoFile, err := os.OpenFile(filepath.Join(outputPath, "tier10_no_info.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		badFile.Close()
		goodFile.Close()
		goodNoInfoFile.Close()
		tier10File.Close()
		return nil, fmt.Errorf("failed to open tier10_no_info.txt: %w", err)
	}

	mergeRequiredFile, err := os.OpenFile(filepath.Join(outputPath, "merge_required.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		badFile.Close()
		goodFile.Close()
		goodNoInfoFile.Close()
		tier10File.Close()
		tier10NoInfoFile.Close()
		return nil, fmt.Errorf("failed to open merge_required.txt: %w", err)
	}

	errorsFile, err := os.OpenFile(filepath.Join(outputPath, "errors.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		badFile.Close()
		goodFile.Close()
		goodNoInfoFile.Close()
		tier10File.Close()
		tier10NoInfoFile.Close()
		mergeRequiredFile.Close()
		return nil, fmt.Errorf("failed to open errors.txt: %w", err)
	}

	return &ResultManager{
		badFile:                badFile,
		mergeRequiredFile:      mergeRequiredFile,
		errorsFile:             errorsFile,
		goodFile:               goodFile,
		goodNoInfoFile:         goodNoInfoFile,
		tier10File:             tier10File,
		tier10NoInfoFile:       tier10NoInfoFile,
		realmFiles:             make(map[string]*os.File),
		realmNoInfoFiles:       make(map[string]*os.File),
		realmTier10Files:       make(map[string]*os.File),
		realmTier10NoInfoFiles: make(map[string]*os.File),
		outputPath:             outputPath,
	}, nil
}

func (rm *ResultManager) AddResult(result Result) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.results = append(rm.results, result)

	if result.Status == Good {
		fmt.Fprintf(rm.goodNoInfoFile, "%s:%s\n", result.Email, result.Password)

		if len(result.Vehicles) == 0 {
			return
		}

		hasTier10 := false
		for _, vehicle := range result.Vehicles {
			if vehicle.Tier == 10 {
				hasTier10 = true
				break
			}
		}

		sort.Slice(result.Vehicles, func(i, j int) bool {
			return result.Vehicles[i].Tier > result.Vehicles[j].Tier
		})

		fmt.Fprintf(rm.goodFile, "=============\n")
		fmt.Fprintf(rm.goodFile, "%s:%s\n\n", result.Email, result.Password)

		for _, vehicle := range result.Vehicles {
			fmt.Fprintf(rm.goodFile, "%s - %d\n", vehicle.Name, vehicle.Tier)
		}
		fmt.Fprintf(rm.goodFile, "=============\n\n")

		if hasTier10 {
			fmt.Fprintf(rm.tier10NoInfoFile, "%s:%s\n", result.Email, result.Password)

			fmt.Fprintf(rm.tier10File, "=============\n")
			fmt.Fprintf(rm.tier10File, "%s:%s\n\n", result.Email, result.Password)

			for _, vehicle := range result.Vehicles {
				fmt.Fprintf(rm.tier10File, "%s - %d\n", vehicle.Name, vehicle.Tier)
			}
			fmt.Fprintf(rm.tier10File, "=============\n\n")
		}

		if result.GameRealm != "" {
			realmNoInfoFile, err := rm.getRealmNoInfoFile(result.GameRealm)
			if err == nil {
				fmt.Fprintf(realmNoInfoFile, "%s:%s\n", result.Email, result.Password)
			}

			realmFile, err := rm.getRealmFile(result.GameRealm)
			if err == nil {
				fmt.Fprintf(realmFile, "=============\n")
				fmt.Fprintf(realmFile, "%s:%s\n\n", result.Email, result.Password)

				for _, vehicle := range result.Vehicles {
					fmt.Fprintf(realmFile, "%s - %d\n", vehicle.Name, vehicle.Tier)
				}
				fmt.Fprintf(realmFile, "=============\n\n")
			}

			if hasTier10 {
				realmTier10NoInfoFile, err := rm.getRealmTier10NoInfoFile(result.GameRealm)
				if err == nil {
					fmt.Fprintf(realmTier10NoInfoFile, "%s:%s\n", result.Email, result.Password)
				}

				realmTier10File, err := rm.getRealmTier10File(result.GameRealm)
				if err == nil {
					fmt.Fprintf(realmTier10File, "=============\n")
					fmt.Fprintf(realmTier10File, "%s:%s\n\n", result.Email, result.Password)

					for _, vehicle := range result.Vehicles {
						fmt.Fprintf(realmTier10File, "%s - %d\n", vehicle.Name, vehicle.Tier)
					}
					fmt.Fprintf(realmTier10File, "=============\n\n")
				}
			}
		}
	} else if result.Status == Bad {
		fmt.Fprintf(rm.badFile, "%s:%s\n", result.Email, result.Password)
	} else if result.Status == MergeRequired {
		fmt.Fprintf(rm.mergeRequiredFile, "%s:%s\n", result.Email, result.Password)
	} else if result.Status == Error {
		fmt.Fprintf(rm.errorsFile, "%s:%s\n", result.Email, result.Password)
	}
}

func (rm *ResultManager) getRealmFile(realm string) (*os.File, error) {
	if file, ok := rm.realmFiles[realm]; ok {
		return file, nil
	}

	realmDir := filepath.Join(rm.outputPath, realm)
	if err := os.MkdirAll(realmDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create realm directory: %w", err)
	}

	file, err := os.OpenFile(filepath.Join(realmDir, "good.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	rm.realmFiles[realm] = file
	return file, nil
}

func (rm *ResultManager) getRealmNoInfoFile(realm string) (*os.File, error) {
	if file, ok := rm.realmNoInfoFiles[realm]; ok {
		return file, nil
	}

	realmDir := filepath.Join(rm.outputPath, realm)
	if err := os.MkdirAll(realmDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create realm directory: %w", err)
	}

	file, err := os.OpenFile(filepath.Join(realmDir, "good_no_info.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	rm.realmNoInfoFiles[realm] = file
	return file, nil
}

func (rm *ResultManager) getRealmTier10File(realm string) (*os.File, error) {
	if file, ok := rm.realmTier10Files[realm]; ok {
		return file, nil
	}

	realmDir := filepath.Join(rm.outputPath, realm)
	if err := os.MkdirAll(realmDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create realm directory: %w", err)
	}

	file, err := os.OpenFile(filepath.Join(realmDir, "tier10.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	rm.realmTier10Files[realm] = file
	return file, nil
}

func (rm *ResultManager) getRealmTier10NoInfoFile(realm string) (*os.File, error) {
	if file, ok := rm.realmTier10NoInfoFiles[realm]; ok {
		return file, nil
	}

	realmDir := filepath.Join(rm.outputPath, realm)
	if err := os.MkdirAll(realmDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create realm directory: %w", err)
	}

	file, err := os.OpenFile(filepath.Join(realmDir, "tier10_no_info.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	rm.realmTier10NoInfoFiles[realm] = file
	return file, nil
}

func (rm *ResultManager) Close() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var errors []error

	if err := rm.badFile.Close(); err != nil {
		errors = append(errors, err)
	}

	if err := rm.goodFile.Close(); err != nil {
		errors = append(errors, err)
	}

	if err := rm.goodNoInfoFile.Close(); err != nil {
		errors = append(errors, err)
	}

	if err := rm.tier10File.Close(); err != nil {
		errors = append(errors, err)
	}

	if err := rm.tier10NoInfoFile.Close(); err != nil {
		errors = append(errors, err)
	}

	for _, file := range rm.realmFiles {
		if err := file.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	for _, file := range rm.realmNoInfoFiles {
		if err := file.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	for _, file := range rm.realmTier10Files {
		if err := file.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	for _, file := range rm.realmTier10NoInfoFiles {
		if err := file.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to close one or more files")
	}

	return nil
}
