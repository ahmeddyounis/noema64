package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

func main() {
	requireSignature := flag.Bool("require-signature", os.Getenv("NOEMA64_REQUIRE_SIGNATURE") == "1", "fail when a platform signature cannot be verified")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: noema64-releasecheck [flags] <artifact>...")
		os.Exit(2)
	}
	for _, path := range flag.Args() {
		report, err := checkArtifact(path, *requireSignature)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(report)
	}
}

func checkArtifact(path string, requireSignature bool) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("release artifact %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("release artifact %s is a directory", path)
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return "", err
	}
	signature := "signature=skipped"
	if runtime.GOOS == "darwin" {
		if err := exec.Command("codesign", "--verify", "--deep", "--strict", path).Run(); err != nil {
			if requireSignature {
				return "", fmt.Errorf("codesign verify %s: %w", path, err)
			}
		} else {
			signature = "signature=verified"
		}
	} else if requireSignature {
		return "", fmt.Errorf("signature verification is not supported on %s for %s", runtime.GOOS, path)
	}
	return fmt.Sprintf("%s sha256=%s %s", path, sum, signature), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open release artifact %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash release artifact %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
