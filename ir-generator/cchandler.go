package ir_generator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

type CCStyle int

const (
	CMakeStyle CCStyle = iota
	BearStyle
)

type CompilerCommand interface {
	SplitArgs() []string

	Run() error
	AddFlags(flags ...string)
	DropFlags(flags ...string)
	ReplaceCompiler(newcompiler string)
	ReplaceTargetExt(newext string)
	SwitchToO0()
	SwitchToC99()

	GetFile() string
	GetTarget() string
	GetDirectory() string
	String() string
}

type CompilerDatabase struct {
	Opt
	Commands            []CompilerCommand
	TopDir              string
	SolveHeaderNotFound bool
	SkipFailed          bool

	Style CCStyle

	taskMutex sync.Mutex
	failFiles map[string]bool
}

func (d *CompilerDatabase) GetAllExistTargets() []string {
	var result []string
	for _, c := range d.Commands {
		tgt := c.GetTarget()
		if _, err := os.Stat(tgt); os.IsNotExist(err) {
			continue
		}

		result = append(result, tgt)
	}
	return result
}

func (d *CompilerDatabase) LLVMLink(llvmlink string, output string) error {
	targets := d.GetAllExistTargets()
	args := []string{
		llvmlink,
	}
	args = append(args, targets...)
	args = append(args, "-o", output)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

func (d *CompilerDatabase) EmitLLVM(clang string) {
	flags := []string{
		"-emit-llvm",
		"-g",
		"-Wno-shift-count-negative",
		"-Wno-division-by-zero",
		"-fno-inline-functions",
		"-Wno-ignored-optimization-argument",
		"-Xclang",
		"-disable-O0-optnone",
		//"-Xclang",
		//"-disable-llvm-optzns",
		"-flto",
		"-Xclang",
		"-disable-llvm-passes",
		"-Wno-everything",
	}

	for i := range d.Commands {
		d.Commands[i].ReplaceCompiler(clang)
		d.Commands[i].ReplaceTargetExt(".bc")
		// d.Commands[i].SwitchToO0()
		// d.Commands[i].SwitchToC99()
		d.Commands[i].AddFlags(flags...)
	}
}

func (d *CompilerDatabase) EmitClangAST(clang string) {
	flags := []string{
		"-emit-ast",
		"-g",
		"-Wno-shift-count-negative",
		"-Wno-division-by-zero",
		"-fno-inline-functions",
		"-Wno-ignored-optimization-argument",
		"-Xclang",
		"-disable-O0-optnone",
		"-Wno-everything",
	}

	for i := range d.Commands {
		d.Commands[i].ReplaceCompiler(clang)
		d.Commands[i].ReplaceTargetExt(".ast")
		// d.Commands[i].SwitchToO0()
		// d.Commands[i].SwitchToC99()
		d.Commands[i].AddFlags(flags...)
	}
}

func (d *CompilerDatabase) dumpStatus() {
	d.taskMutex.Lock()
	for k := range d.failFiles {
		fmt.Println("Failed", k)
	}
	failed := float32(len(d.failFiles))
	d.taskMutex.Unlock()

	total := float32(len(d.Commands))
	fmt.Printf("Compilation success rate: %.2f%%\n", (total-failed)/total*100)
}

func (d *CompilerDatabase) Dump() {
	b, err := json.MarshalIndent(d.Commands, "", "    ")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(b))
}

func (d *CompilerDatabase) markFailed(tgt string) {
	d.taskMutex.Lock()
	d.failFiles[tgt] = true
	d.taskMutex.Unlock()
}

func (d *CompilerDatabase) run(c CompilerCommand) {
	err := c.Run()
	if err != nil && !d.SkipFailed {
		args := c.SplitArgs()
		fmt.Println("Failed commands:")
		for _, a := range args {
			fmt.Println(a)
		}

		log.Println(c)
		log.Fatalln(err)
	}

	target := c.GetTarget()
	directory := c.GetDirectory()

	if err != nil {
		d.markFailed(target)
		return
	}

	if !d.Opt.NeedRun() {
		return
	}

	if err = d.Opt.Run(target, directory); err != nil {
		d.markFailed(target)
		return
	}
}

func (d *CompilerDatabase) Run() {
	fmt.Println()
	total := len(d.Commands)
	for i := 0; i < total; i++ {
		fmt.Printf("processing [%d/%d]\n", i+1, total)
		d.run(d.Commands[i])
	}

	if d.SkipFailed {
		d.dumpStatus()
	}
}

func (d *CompilerDatabase) RunOnly(file string) {
	fmt.Println()
	for i := 0; i < len(d.Commands); i++ {
		cmd := d.Commands[i]
		if strings.HasSuffix(cmd.GetFile(), file) {
			fmt.Println(cmd)
			d.run(cmd)
		}
	}
}

func (d *CompilerDatabase) RunParallel() {
	jobs := runtime.NumCPU() / 2
	taskCh := make(chan CompilerCommand, jobs)
	total := len(d.Commands)

	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < jobs; i++ {
		go func() {
			for task := range taskCh {
				d.run(task)
				wg.Done()
			}
		}()
	}

	fmt.Println()
	for i := 0; i < total; i++ {
		taskCh <- d.Commands[i]
		fmt.Printf("processing [%d/%d]\n", i+1, total)
	}

	close(taskCh)
	wg.Wait()

	if d.SkipFailed {
		d.dumpStatus()
	}
}

func (d *CompilerDatabase) Rewrite(ccjson string) {
	b, err := json.MarshalIndent(d, "", "    ")
	if err != nil {
		log.Fatalln(err)
	}

	err = os.WriteFile(ccjson, b, 0644)
	if err != nil {
		log.Fatalln(err)
	}
}

func (d *CompilerDatabase) Load(ccjson string) {
	b, err := os.ReadFile(ccjson)
	if err != nil {
		log.Fatalln(err)
	}

	fileSet := make(map[string]bool)

	switch d.Style {
	case CMakeStyle:
		var commands []CMakeCC
		if err = json.Unmarshal(b, &commands); err != nil {
			log.Fatalln(err)
		}
		for i := range commands {
			file := commands[i].GetTarget()
			if fileSet[file] {
				continue
			}

			d.Commands = append(d.Commands, &commands[i])
			fileSet[file] = true
		}
	case BearStyle:
		var commands []BearCC
		if err = json.Unmarshal(b, &commands); err != nil {
			log.Fatalln(err)
		}
		for i := range commands {
			file := commands[i].GetTarget()
			if fileSet[file] {
				continue
			}

			d.Commands = append(d.Commands, &commands[i])
			fileSet[file] = true
		}
	}
}

func NewCompilerDataBase() *CompilerDatabase {
	return &CompilerDatabase{
		failFiles: make(map[string]bool),
	}
}
