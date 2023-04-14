package ir_generator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
)

type CCStyle int

const (
	CMakeStyle CCStyle = iota
	CCJSONStyle
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
	EscapeQuotes()

	GetFile() string
	GetTarget() string
}

type CompilerDatabase struct {
	Commands            []CompilerCommand
	TopDir              string
	SolveHeaderNotFound bool
	SkipFailed          bool
	Style               CCStyle

	taskMutex sync.Mutex
	failFiles map[string]bool
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
		"-Wno-everything",
	}

	for i := range d.Commands {
		d.Commands[i].ReplaceCompiler(clang)
		d.Commands[i].ReplaceTargetExt(".bc")
		d.Commands[i].SwitchToO0()
		// d.Commands[i].SwitchToC99()
		d.Commands[i].AddFlags(flags...)
		d.Commands[i].EscapeQuotes()
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
		d.Commands[i].SwitchToO0()
		// d.Commands[i].SwitchToC99()
		d.Commands[i].AddFlags(flags...)
		d.Commands[i].EscapeQuotes()
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

func (d *CompilerDatabase) run(c CompilerCommand) {
	err := c.Run()
	if err != nil && !d.SkipFailed {
		log.Fatalln(err)
	}

	if err != nil {
		d.taskMutex.Lock()
		d.failFiles[c.GetTarget()] = true
		d.taskMutex.Unlock()
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
	case CCJSONStyle:
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
