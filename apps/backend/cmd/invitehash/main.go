// Command invitehash turns an invite code into the bcrypt hash that REGISTER_INVITE_CODE_HASH
// expects, so the code itself never has to be stored anywhere.
//
//	go run ./cmd/invitehash
//
// The code is read from the terminal rather than taken as an argument, because an argument
// would land in the shell history and in the process list.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// cost matches the one used for passwords: slow enough to blunt offline guessing, fast enough
// that a single check adds nothing noticeable to a request.
const cost = 12

// bcrypt silently ignores anything past 72 bytes, so a longer code would appear to work while
// only its first 72 bytes actually mattered.
const maxCodeBytes = 72

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Fprint(os.Stderr, "Invite code: ")

	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && strings.TrimSpace(code) == "" {
		return fmt.Errorf("read code: %w", err)
	}
	// The service trims the submitted code the same way, so the hash has to be of the trimmed
	// value or a code typed with a trailing space would never match.
	code = strings.TrimSpace(code)

	if code == "" {
		return fmt.Errorf("the code is empty")
	}
	if len(code) > maxCodeBytes {
		return fmt.Errorf("the code is %d bytes; bcrypt ignores anything past %d", len(code), maxCodeBytes)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(code), cost)
	if err != nil {
		return fmt.Errorf("hash code: %w", err)
	}

	// Only the hash goes to stdout, so it can be piped straight into a secret store without
	// the code following it.
	fmt.Println(string(hash))

	fmt.Fprintln(os.Stderr, "\nSet it as REGISTER_INVITE_CODE_HASH, and remove REGISTER_INVITE_CODE.")
	fmt.Fprintln(os.Stderr, "Keep the code itself in a password manager: this hash cannot be reversed.")
	return nil
}
