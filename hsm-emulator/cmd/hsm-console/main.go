package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/your-org/hsm-emulator/internal/config"
	"github.com/your-org/hsm-emulator/internal/lmk"
)

/*
 * HSM Console — mimics the production PayShield 10K serial/SSH console.
 *
 * Production configuration flow:
 *   1. Connect to console port (serial or SSH, default port 2222)
 *   2. Login with administrator credentials
 *   3. Configure network settings
 *   4. Load or generate LMK keys
 *   5. Configure host interface (port, header length, max connections)
 *   6. Authorize HSM
 *   7. HSM begins processing host commands
 *
 * This tool generates/manages the config.json and lmk.bin files
 * used by the hsm-emulator server.
 */

func main() {
	cfgPath := flag.String("config", "config.json", "path to config JSON")
	lmkPath := flag.String("lmk", "lmk.bin", "path to LMK binary file")
	flag.Parse()

	cfg := config.DefaultConfig()
	if existing, err := config.Load(*cfgPath); err == nil {
		cfg = existing
		fmt.Println("Loaded existing configuration from", *cfgPath)
	}

	lmkStore := lmk.New()
	if err := lmkStore.LoadFromFile(*lmkPath); err == nil {
		cfg.SetLMKLoaded(true)
		fmt.Println("Loaded existing LMK from", *lmkPath)
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println("  Thales PayShield 10K — Configuration Console")
	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println()

	for {
		printMainMenu(cfg)
		fmt.Print("\nSelect option: ")
		if !scanner.Scan() {
			break
		}
		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			menuSecurity(scanner, cfg, lmkStore, *lmkPath, *cfgPath)
		case "2":
			menuHostInterface(scanner, cfg, *cfgPath)
		case "3":
			menuNetwork(scanner, cfg, *cfgPath)
		case "4":
			menuHSMIdentity(scanner, cfg, *cfgPath)
		case "5":
			menuStatus(cfg, lmkStore)
		case "6":
			menuCommands(scanner, cfg, *cfgPath)
		case "0", "q", "quit", "exit":
			fmt.Println("\nConsole session ended.")
			return
		default:
			fmt.Println("Invalid option.")
		}
	}
}

func printMainMenu(cfg *config.Config) {
	authStr := "NOT AUTHORIZED"
	if cfg.IsAuthorized() {
		authStr = "AUTHORIZED"
	}
	lmkStr := "NOT LOADED"
	if cfg.IsLMKLoaded() {
		lmkStr = "LOADED"
	}

	fmt.Println()
	fmt.Printf("  HSM Status: %s | LMK: %s\n", authStr, lmkStr)
	fmt.Println("  ──────────────────────────────────────────")
	fmt.Println("  1. Security (LMK, Authorization)")
	fmt.Println("  2. Host Interface Settings")
	fmt.Println("  3. Network Settings")
	fmt.Println("  4. HSM Identity")
	fmt.Println("  5. Display Status")
	fmt.Println("  6. Command Management")
	fmt.Println("  0. Exit")
}

// ── Security Menu ────────────────────────────────────────────

func menuSecurity(scanner *bufio.Scanner, cfg *config.Config, store *lmk.Store, lmkPath, cfgPath string) {
	for {
		fmt.Println()
		fmt.Println("  ── Security ──────────────────────────────")
		fmt.Println("  1. Generate New LMK Set")
		fmt.Println("  2. Load Test LMK (deterministic)")
		fmt.Println("  3. Load LMK from Smart Card Components")
		fmt.Println("  4. Print LMK Check Values")
		fmt.Println("  5. Authorize HSM")
		fmt.Println("  6. Deauthorize HSM")
		fmt.Println("  7. Export LMK to File")
		fmt.Println("  0. Back")
		fmt.Print("\n  Select: ")
		if !scanner.Scan() {
			return
		}

		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			fmt.Println("\n  Generating random LMK set...")
			if err := store.GenerateRandom(); err != nil {
				fmt.Printf("  ERROR: %v\n", err)
				continue
			}
			cfg.SetLMKLoaded(true)
			if err := store.SaveToFile(lmkPath); err != nil {
				fmt.Printf("  WARNING: Could not save LMK: %v\n", err)
			} else {
				fmt.Printf("  LMK saved to %s\n", lmkPath)
			}
			cfg.Save(cfgPath)
			printCheckValues(store)

		case "2":
			fmt.Println("\n  Loading well-known test LMK set...")
			fmt.Println("  WARNING: Test LMK is deterministic and NOT suitable for production!")
			store.LoadTestLMK()
			cfg.SetLMKLoaded(true)
			if err := store.SaveToFile(lmkPath); err != nil {
				fmt.Printf("  WARNING: Could not save LMK: %v\n", err)
			}
			cfg.Save(cfgPath)
			printCheckValues(store)

		case "3":
			loadFromComponents(scanner, store, cfg, lmkPath, cfgPath)

		case "4":
			printCheckValues(store)

		case "5":
			if !cfg.IsLMKLoaded() {
				fmt.Println("\n  ERROR: Cannot authorize — LMK not loaded")
				continue
			}
			fmt.Print("\n  Enter authorization code (or 'confirm'): ")
			if !scanner.Scan() {
				return
			}
			code := strings.TrimSpace(scanner.Text())
			if code == "confirm" || code == "CONFIRM" || len(code) >= 4 {
				cfg.SetAuthorized(true)
				cfg.Save(cfgPath)
				fmt.Println("  HSM AUTHORIZED — ready to process host commands")
			} else {
				fmt.Println("  Authorization cancelled")
			}

		case "6":
			cfg.SetAuthorized(false)
			cfg.Save(cfgPath)
			fmt.Println("\n  HSM DEAUTHORIZED — host commands will be rejected")

		case "7":
			if !store.IsLoaded() {
				fmt.Println("\n  ERROR: No LMK loaded")
				continue
			}
			fmt.Print("  Save path [lmk.bin]: ")
			if !scanner.Scan() {
				return
			}
			path := strings.TrimSpace(scanner.Text())
			if path == "" {
				path = lmkPath
			}
			if err := store.SaveToFile(path); err != nil {
				fmt.Printf("  ERROR: %v\n", err)
			} else {
				fmt.Printf("  LMK exported to %s\n", path)
			}

		case "0":
			return
		}
	}
}

func loadFromComponents(scanner *bufio.Scanner, store *lmk.Store, cfg *config.Config, lmkPath, cfgPath string) {
	fmt.Println()
	fmt.Println("  ── Load LMK from Smart Card Components ──")
	fmt.Println("  Production PayShield requires 3 key custodians,")
	fmt.Println("  each contributing a component via smart card.")
	fmt.Println("  Components are XORed to form the final LMK set.")
	fmt.Println()

	components := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		fmt.Printf("  Custodian %d — Enter component hex (%d bytes = %d hex chars)\n", i+1, lmk.TotalLMKLen, lmk.TotalLMKLen*2)
		fmt.Printf("  Or type 'random' to generate: ")
		if !scanner.Scan() {
			return
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "random" || input == "" {
			comp := make([]byte, lmk.TotalLMKLen)
			rand.Read(comp)
			components[i] = comp
			fmt.Printf("  Generated component %d: %s...\n", i+1, hex.EncodeToString(comp[:16]))
		} else {
			comp, err := hex.DecodeString(input)
			if err != nil || len(comp) < lmk.TotalLMKLen {
				fmt.Printf("  ERROR: Need %d hex chars of valid hex\n", lmk.TotalLMKLen*2)
				return
			}
			components[i] = comp
		}
	}

	if err := store.LoadFromComponents(components[0], components[1], components[2]); err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}

	cfg.SetLMKLoaded(true)
	store.SaveToFile(lmkPath)
	cfg.Save(cfgPath)
	fmt.Println("  LMK loaded from 3 components")
	printCheckValues(store)
}

func printCheckValues(store *lmk.Store) {
	kcvs, err := store.CheckValues()
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}
	fmt.Println("\n  LMK Check Values:")
	fmt.Println("  ─────────────────────────")
	for i, kcv := range kcvs {
		fmt.Printf("  LMK %02d-%02d: %s\n", i*2, i*2+1, string(kcv[:]))
	}
}

// ── Host Interface Menu ──────────────────────────────────────

func menuHostInterface(scanner *bufio.Scanner, cfg *config.Config, cfgPath string) {
	for {
		fmt.Println()
		fmt.Println("  ── Host Interface ────────────────────────")
		fmt.Printf("  1. TCP Port:         %d\n", cfg.HostPort)
		fmt.Printf("  2. Header Length:    %d bytes\n", cfg.HeaderLength)
		fmt.Printf("  3. Max Connections:  %d\n", cfg.MaxConnections)
		fmt.Printf("  4. Command Timeout:  %d seconds\n", cfg.CommandTimeout)
		fmt.Println("  0. Back")
		fmt.Print("\n  Select: ")
		if !scanner.Scan() {
			return
		}

		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			fmt.Print("  New TCP port [1500]: ")
			if scanner.Scan() {
				if v, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil && v > 0 && v < 65536 {
					cfg.HostPort = v
					cfg.Save(cfgPath)
					fmt.Printf("  Port set to %d\n", v)
				}
			}
		case "2":
			fmt.Print("  Header length (2 or 4): ")
			if scanner.Scan() {
				if v, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil && (v == 2 || v == 4) {
					cfg.HeaderLength = v
					cfg.Save(cfgPath)
					fmt.Printf("  Header length set to %d\n", v)
				}
			}
		case "3":
			fmt.Print("  Max connections: ")
			if scanner.Scan() {
				if v, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil && v > 0 {
					cfg.MaxConnections = v
					cfg.Save(cfgPath)
					fmt.Printf("  Max connections set to %d\n", v)
				}
			}
		case "4":
			fmt.Print("  Command timeout (seconds): ")
			if scanner.Scan() {
				if v, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil && v > 0 {
					cfg.CommandTimeout = v
					cfg.Save(cfgPath)
					fmt.Printf("  Timeout set to %d seconds\n", v)
				}
			}
		case "0":
			return
		}
	}
}

// ── Network Menu ─────────────────────────────────────────────

func menuNetwork(scanner *bufio.Scanner, cfg *config.Config, cfgPath string) {
	for {
		fmt.Println()
		fmt.Println("  ── Network Settings ──────────────────────")
		fmt.Printf("  1. Bind Address:     %s\n", cfg.HostAddress)
		fmt.Printf("  2. Console Port:     %d\n", cfg.ConsolePort)
		fmt.Println("  0. Back")
		fmt.Print("\n  Select: ")
		if !scanner.Scan() {
			return
		}

		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			fmt.Print("  Bind address [0.0.0.0]: ")
			if scanner.Scan() {
				v := strings.TrimSpace(scanner.Text())
				if v != "" {
					cfg.HostAddress = v
					cfg.Save(cfgPath)
					fmt.Printf("  Bind address set to %s\n", v)
				}
			}
		case "2":
			fmt.Print("  Console port [2222]: ")
			if scanner.Scan() {
				if v, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil && v > 0 {
					cfg.ConsolePort = v
					cfg.Save(cfgPath)
					fmt.Printf("  Console port set to %d\n", v)
				}
			}
		case "0":
			return
		}
	}
}

// ── HSM Identity Menu ────────────────────────────────────────

func menuHSMIdentity(scanner *bufio.Scanner, cfg *config.Config, cfgPath string) {
	for {
		fmt.Println()
		fmt.Println("  ── HSM Identity ──────────────────────────")
		fmt.Printf("  1. Model:            %s\n", cfg.Model)
		fmt.Printf("  2. Serial Number:    %s\n", cfg.SerialNumber)
		fmt.Printf("  3. Firmware Version: %s\n", cfg.FirmwareVersion)
		fmt.Printf("  4. LMK Identifier:   %d\n", cfg.LMKIdentifier)
		fmt.Println("  0. Back")
		fmt.Print("\n  Select: ")
		if !scanner.Scan() {
			return
		}

		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			fmt.Print("  Model name: ")
			if scanner.Scan() {
				v := strings.TrimSpace(scanner.Text())
				if v != "" {
					cfg.Model = v
					cfg.Save(cfgPath)
				}
			}
		case "2":
			fmt.Print("  Serial number: ")
			if scanner.Scan() {
				v := strings.TrimSpace(scanner.Text())
				if v != "" {
					cfg.SerialNumber = v
					cfg.Save(cfgPath)
				}
			}
		case "3":
			fmt.Print("  Firmware version: ")
			if scanner.Scan() {
				v := strings.TrimSpace(scanner.Text())
				if v != "" {
					cfg.FirmwareVersion = v
					cfg.Save(cfgPath)
				}
			}
		case "4":
			fmt.Print("  LMK identifier (0-9): ")
			if scanner.Scan() {
				if v, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil && v >= 0 && v <= 9 {
					cfg.LMKIdentifier = v
					cfg.Save(cfgPath)
				}
			}
		case "0":
			return
		}
	}
}

// ── Status Display ───────────────────────────────────────────

func menuStatus(cfg *config.Config, store *lmk.Store) {
	authStr := "NOT AUTHORIZED"
	if cfg.IsAuthorized() {
		authStr = "AUTHORIZED"
	}
	lmkStr := "NOT LOADED"
	if cfg.IsLMKLoaded() {
		lmkStr = "LOADED"
	}

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════╗")
	fmt.Println("  ║         HSM Status Summary                 ║")
	fmt.Println("  ╠════════════════════════════════════════════╣")
	fmt.Printf("  ║  Model:       %-28s ║\n", cfg.Model)
	fmt.Printf("  ║  Serial:      %-28s ║\n", cfg.SerialNumber)
	fmt.Printf("  ║  Firmware:    %-28s ║\n", cfg.FirmwareVersion)
	fmt.Printf("  ║  Host Port:   %-28d ║\n", cfg.HostPort)
	fmt.Printf("  ║  Header Len:  %-28d ║\n", cfg.HeaderLength)
	fmt.Printf("  ║  Max Conns:   %-28d ║\n", cfg.MaxConnections)
	fmt.Printf("  ║  LMK Status:  %-28s ║\n", lmkStr)
	fmt.Printf("  ║  Auth Status: %-28s ║\n", authStr)
	fmt.Printf("  ║  LMK ID:      %-28d ║\n", cfg.LMKIdentifier)
	fmt.Printf("  ║  Audit:       %-28v ║\n", cfg.AuditEnabled)
	fmt.Println("  ╚════════════════════════════════════════════╝")

	if store.IsLoaded() {
		printCheckValues(store)
	}
}

// ── Command Management ───────────────────────────────────────

func menuCommands(scanner *bufio.Scanner, cfg *config.Config, cfgPath string) {
	for {
		fmt.Println()
		fmt.Println("  ── Command Management ────────────────────")
		fmt.Println("  1. Disable a command")
		fmt.Println("  2. Enable a command")
		fmt.Println("  3. List disabled commands")
		fmt.Printf("  4. Audit logging: %v\n", cfg.AuditEnabled)
		fmt.Println("  0. Back")
		fmt.Print("\n  Select: ")
		if !scanner.Scan() {
			return
		}

		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			fmt.Print("  Command code to disable (e.g. EI): ")
			if scanner.Scan() {
				cmd := strings.ToUpper(strings.TrimSpace(scanner.Text()))
				if len(cmd) == 2 {
					if cfg.DisabledCommands == nil {
						cfg.DisabledCommands = make(map[string]bool)
					}
					cfg.DisabledCommands[cmd] = true
					cfg.Save(cfgPath)
					fmt.Printf("  Command %s disabled\n", cmd)
				}
			}
		case "2":
			fmt.Print("  Command code to enable: ")
			if scanner.Scan() {
				cmd := strings.ToUpper(strings.TrimSpace(scanner.Text()))
				delete(cfg.DisabledCommands, cmd)
				cfg.Save(cfgPath)
				fmt.Printf("  Command %s enabled\n", cmd)
			}
		case "3":
			if len(cfg.DisabledCommands) == 0 {
				fmt.Println("  No commands disabled")
			} else {
				fmt.Println("  Disabled commands:")
				for cmd := range cfg.DisabledCommands {
					fmt.Printf("    %s\n", cmd)
				}
			}
		case "4":
			cfg.AuditEnabled = !cfg.AuditEnabled
			cfg.Save(cfgPath)
			fmt.Printf("  Audit logging: %v\n", cfg.AuditEnabled)
		case "0":
			return
		}
	}
}
