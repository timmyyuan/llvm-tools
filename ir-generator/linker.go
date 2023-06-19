package ir_generator

import (
	"os"
	"os/exec"
)

type Linker struct {
	Name            string
	Output          string
	DisableOverride bool
	Targets         []string
}

func NewLLVMLinker() *Linker {
	return &Linker{
		Name: "llvm-link",
	}
}

func (l *Linker) Link() error {
	args := []string{
		l.Name,
		"--internalize",
	}

	targets := l.Targets

	if !l.DisableOverride {
		// left the last bitcode to be the input of llvm-linker
		for i := 0; i < len(targets)-1; i++ {
			targets[i] = "-override=" + targets[i]
		}
	}

	args = append(args, targets...)
	args = append(args, "-o", l.Output)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
