package ir_generator

import (
	"os"
	"os/exec"
)

type LLVMDis struct {
	Name   string
	Input  string
	Output string
}

func NewDefaultLLVMDis(input, output string) *LLVMDis {
	return &LLVMDis{
		Name:   "llvm-dis",
		Input:  input,
		Output: output,
	}
}

func (d *LLVMDis) NeedRun() bool {
	if _, err := exec.LookPath(d.Name); err != nil {
		return false
	}

	if _, err := os.Stat(d.Input); os.IsNotExist(err) {
		return false
	}

	return true
}

func (d *LLVMDis) Run() error {
	runArgs := func(args []string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		return cmd.Run()
	}

	args := []string{d.Name}
	args = append(args, d.Input, "-o", d.Output)

	if err := runArgs(args); err != nil {
		return err
	}

	return nil
}
