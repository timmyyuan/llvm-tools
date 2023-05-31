package ir_generator

import (
	"os"
	"os/exec"
	"strings"
)

type Opt struct {
	passOptions            []string
	analysisOptions        []string
	EnableOptModuleSummary bool
	EnableOptMem2Reg       bool
}

func (o *Opt) initOptions() {
	if o.EnableOptModuleSummary {
		o.analysisOptions = append(o.analysisOptions, "-module-summary")
		o.passOptions = append(o.passOptions, "canonicalize-aliases")
		o.passOptions = append(o.passOptions, "name-anon-globals")
	}

	if o.EnableOptMem2Reg {
		o.passOptions = append(o.passOptions, "mem2reg")
	}
}

func (o *Opt) NeedRun() bool {
	return o.EnableOptModuleSummary || o.EnableOptMem2Reg
}

func (o *Opt) Run(target, directory string) error {
	o.initOptions()

	runArgs := func(args []string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Dir = directory

		return cmd.Run()
	}

	args := []string{"opt"}
	args = append(args, o.analysisOptions...)
	args = append(args, "-passes="+strings.Join(o.passOptions, ","))
	args = append(args, target, "-o", target)

	if err := runArgs(args); err != nil {
		return err
	}

	return nil
}
