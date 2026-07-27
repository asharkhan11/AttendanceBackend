package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/school/attendance-backend/internal/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: pwtool <genpass|hash> [password]")
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "genpass":
		// Default 16 char random password
		length := 16
		if len(os.Args) > 2 {
			if l, err := strconv.Atoi(os.Args[2]); err == nil && l > 0 {
				length = l
			}
		}
		pass, err := auth.GenerateRandomPassword(length)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error generating password: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(pass)

	case "hash":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: pwtool hash <password>")
			os.Exit(1)
		}
		pass := os.Args[2]
		// Argon2id parameters: memory=65536KB, iterations=3, parallelism=2
		passSvc := auth.NewPasswordService(65536, 3, 2)
		hash, err := passSvc.HashPassword(pass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error hashing password: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(hash)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
