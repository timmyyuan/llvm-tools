package ostools

import (
	"strings"
	"os"
	"os/exec"
)

type FileType int

const (
	Bitcode FileType = iota
	Unknown
)

func convertFileType(output string) FileType {
	if strings.Contains(output, "LLVM IR bitcode") {
		return Bitcode
	}

	return Unknown
}  

func GetFileType(path string) FileType {
	cmd := exec.Command("file", path)
	cmd.Stderr = os.Stderr

	b, _ := cmd.Output()
	return convertFileType(string(b))
}

func IsBitcode(path string) bool {
	return GetFileType(path) == Bitcode
}